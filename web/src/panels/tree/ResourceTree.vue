<script setup lang="ts">
import {
  onActivated,
  onDeactivated,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import Tree from "primevue/tree";
import type { TreeNode as PVNode } from "primevue/treenode";
import { fetchDoc, fetchPage } from "@/api/dataSource";
import type {
  DataSource,
  Icon,
  ResourceIdentity,
  Row,
  TreeGroup,
  TreeNode,
} from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";

// A branch can hold an unbounded number of children: a node takes one page on
// expand and every further page needs an explicit "Load more…" click, up to a
// cap past which the branch stops growing.
const CHILD_PAGE_SIZE = 200;
const MAX_CHILDREN = 2000;
// A forced reload replays the pages a branch already holds, up to the server
// page limit, so a refresh does not throw away "Load more…" clicks.
const MAX_REFRESH_PAGE_SIZE = 500;
// A refresh tick fans out over the expanded branches, so it is capped at the
// most recently expanded ones and run a few requests at a time.
const MAX_REFRESH_NODES = 25;
const REFRESH_CONCURRENCY = 4;

interface NodeData {
  isGroup?: boolean;
  isMore?: boolean;
  icon?: Icon;
  ref?: ResourceIdentity;
  row?: Row;
  source?: DataSource;
  resourceKind?: string;
  listParams?: Record<string, string>;
  groupLabel?: string;
  parentPath?: string[];
  nextCursor?: string;
}

interface LoadChildrenOptions {
  force?: boolean;
  loading?: boolean;
}

function nodeQualifier(data: NodeData): string {
  return data.parentPath?.length
    ? data.parentPath.join(" / ")
    : (data.groupLabel ?? "");
}

const props = defineProps<{
  connectionId: string;
  groups: TreeGroup[];
  selectedUid?: string;
  selectedGroup?: string;
  refreshKey?: string;
}>();
const emit = defineEmits<{
  "select-group": [key: string];
  "select-node": [row: Row, qualifier: string];
  "select-list": [kind: string, params?: Record<string, string>];
}>();

const nodes = ref<PVNode[]>([]);
const badges = reactive<Record<string, string | number>>({});
const expandedKeys = ref<Record<string, boolean>>({});
const selectionKeys = ref<Record<string, boolean>>({});
const active = ref(true);
// Sentinel key -> owning node, so a "Load more…" click finds its branch without
// putting a parent back-reference into the reactive node graph.
const moreParents = new Map<string, PVNode>();
const expandSeq = new Map<string, number>();
let expandCounter = 0;
// A refresh tick that lands while the tab is KeepAlive-deactivated is replayed
// on activation instead of being dropped.
let staleRefresh = false;

function toNode(
  n: TreeNode,
  parentPath: string[] = [],
  groupLabel = "",
): PVNode {
  return {
    key: n.key,
    label: n.label,
    leaf: !n.childrenSource,
    data: {
      icon: n.icon,
      ref: n.ref,
      row: { ...n.data, ref: n.ref },
      source: n.childrenSource,
      resourceKind: n.resourceKind,
      listParams: n.listParams,
      groupLabel,
      parentPath,
    },
  };
}

function treeNodeIdentity(n: TreeNode): string {
  return n.ref?.uid ? `ref:${n.ref.uid}` : `key:${n.key}`;
}

function pvNodeIdentity(n: PVNode): string {
  const data = n.data as NodeData;
  return data.ref?.uid ? `ref:${data.ref.uid}` : `key:${String(n.key)}`;
}

function updateNode(
  target: PVNode,
  source: TreeNode,
  parentPath: string[],
  groupLabel: string,
): PVNode {
  const next = toNode(source, parentPath, groupLabel);
  const cursor = (target.data as NodeData | undefined)?.nextCursor;
  target.key = next.key;
  target.label = next.label;
  target.leaf = next.leaf;
  // Children (and their "Load more…" sentinel) survive the update, so the
  // cursor that pages them has to survive with them.
  target.data = next.leaf ? next.data : { ...next.data, nextCursor: cursor };
  if (next.leaf) target.children = undefined;
  return target;
}

function loadedChildren(node: PVNode): PVNode[] {
  return (node.children ?? []).filter(
    (child) => !(child.data as NodeData).isMore,
  );
}

function reconcileChildren(
  node: PVNode,
  items: TreeNode[],
  parentPath: string[],
  groupLabel: string,
  keep: PVNode[] = [],
): PVNode[] {
  const existing = new Map(
    loadedChildren(node).map((child) => [pvNodeIdentity(child), child]),
  );
  const seen = new Set(keep.map((child) => pvNodeIdentity(child)));
  const added = items
    .filter((item) => !seen.has(treeNodeIdentity(item)))
    .map((item) => {
      const child = existing.get(treeNodeIdentity(item));
      return child
        ? updateNode(child, item, parentPath, groupLabel)
        : toNode(item, parentPath, groupLabel);
    });
  return [...keep, ...added];
}

// The sentinel is a leaf row appended after the loaded children; selecting it
// pulls exactly one more page.
function moreNode(node: PVNode): PVNode {
  const key = `${String(node.key)}::more`;
  moreParents.set(key, node);
  return { key, label: "Load more…", leaf: true, data: { isMore: true } };
}

function setChildren(node: PVNode, children: PVNode[], cursor: string): void {
  const data = node.data as NodeData;
  const capped = children.length >= MAX_CHILDREN;
  data.nextCursor = cursor && !capped ? cursor : undefined;
  node.children = data.nextCursor ? [...children, moreNode(node)] : children;
}

function childContext(node: PVNode): {
  parentPath: string[];
  groupLabel: string;
} {
  const data = node.data as NodeData;
  return {
    groupLabel: data.isGroup
      ? String(node.label ?? "")
      : (data.groupLabel ?? ""),
    parentPath: data.isGroup
      ? []
      : [...(data.parentPath ?? []), String(node.label ?? "")],
  };
}

function selectedNodeKey(uid: string): string {
  const found = findNodeByUid(nodes.value, uid);
  return found ? String(found.key) : uid;
}

function findNodeByUid(items: PVNode[], uid: string): PVNode | undefined {
  for (const item of items) {
    const data = item.data as NodeData;
    if (data.ref?.uid === uid) return item;
    const child = item.children ? findNodeByUid(item.children, uid) : undefined;
    if (child) return child;
  }
  return undefined;
}

function resetRootNodes(): void {
  moreParents.clear();
  nodes.value = props.groups.map((g) => ({
    key: g.key,
    label: g.label,
    leaf: !g.source?.routeId,
    data: {
      isGroup: true,
      icon: g.icon,
      source: g.source,
      ref: g.ref,
      row: g.ref ? { ref: g.ref } : undefined,
    },
  }));
}

async function loadChildren(
  node: PVNode,
  options: LoadChildrenOptions = {},
): Promise<void> {
  const data = node.data as NodeData;
  const showLoading = options.loading ?? true;
  if ((node.children && !options.force) || !data.source?.routeId) return;
  if (showLoading) node.loading = true;
  try {
    const page = await fetchPage<TreeNode>(
      props.connectionId,
      data.source,
      {},
      {
        limit: Math.min(
          Math.max(loadedChildren(node).length, CHILD_PAGE_SIZE),
          MAX_REFRESH_PAGE_SIZE,
        ),
      },
    );
    const { parentPath, groupLabel } = childContext(node);
    setChildren(
      node,
      reconcileChildren(node, page.items, parentPath, groupLabel),
      page.nextCursor,
    );
  } finally {
    if (showLoading) node.loading = false;
  }
}

async function loadMoreChildren(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  const cursor = data.nextCursor;
  if (!cursor || !data.source?.routeId || node.loading) return;
  node.loading = true;
  try {
    const page = await fetchPage<TreeNode>(
      props.connectionId,
      data.source,
      {},
      {
        cursor,
        limit: CHILD_PAGE_SIZE,
      },
    );
    const { parentPath, groupLabel } = childContext(node);
    setChildren(
      node,
      reconcileChildren(
        node,
        page.items,
        parentPath,
        groupLabel,
        loadedChildren(node),
      ),
      page.nextCursor,
    );
  } finally {
    node.loading = false;
  }
}

function collectExpanded(items: PVNode[], out: PVNode[]): void {
  for (const node of items) {
    if ((node.data as NodeData).isMore) continue;
    if (!expandedKeys.value[String(node.key)]) continue;
    out.push(node);
    if (node.children?.length) collectExpanded(node.children, out);
  }
}

// Expanded branches in tree order, trimmed to the most recently expanded so a
// deeply explored tree cannot turn one tick into an unbounded fan-out.
function refreshTargets(items: PVNode[]): PVNode[] {
  const expanded: PVNode[] = [];
  collectExpanded(items, expanded);
  if (expanded.length <= MAX_REFRESH_NODES) return expanded;
  const recent = new Set(
    [...expanded]
      .sort(
        (a, b) =>
          (expandSeq.get(String(b.key)) ?? 0) -
          (expandSeq.get(String(a.key)) ?? 0),
      )
      .slice(0, MAX_REFRESH_NODES),
  );
  return expanded.filter((node) => recent.has(node));
}

async function reloadExpanded(
  nodesToReload: PVNode[],
  options: LoadChildrenOptions = {},
): Promise<void> {
  // One wave per depth: children materialised by a wave can themselves be
  // expanded, and they only become visible to refreshTargets once loaded.
  const loaded = new Set<string>();
  while (loaded.size < MAX_REFRESH_NODES) {
    const targets = refreshTargets(nodesToReload).filter(
      (node) => !loaded.has(String(node.key)),
    );
    if (!targets.length) return;
    for (const node of targets) loaded.add(String(node.key));
    let next = 0;
    const worker = async (): Promise<void> => {
      while (next < targets.length) {
        await loadChildren(targets[next++]!, options);
      }
    };
    const workers = Math.min(REFRESH_CONCURRENCY, targets.length);
    await Promise.all(Array.from({ length: workers }, () => worker()));
  }
}

function expandNode(node: PVNode): void {
  expandSeq.set(String(node.key), ++expandCounter);
  expandedKeys.value = { ...expandedKeys.value, [String(node.key)]: true };
}

async function onNodeExpand(node: PVNode): Promise<void> {
  expandNode(node);
  await loadChildren(node);
}

function onNodeCollapse(node: PVNode): void {
  const next = { ...expandedKeys.value };
  delete next[String(node.key)];
  expandedKeys.value = next;
}

async function onNodeSelect(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  if (data.isMore) {
    const parent = moreParents.get(String(node.key));
    if (parent) await loadMoreChildren(parent);
    return;
  }
  selectionKeys.value = { [String(node.key)]: true };
  if (data.isGroup) {
    if (data.ref && data.row)
      emit("select-node", data.row, nodeQualifier(data));
    else emit("select-group", String(node.key));
  } else if (data.resourceKind)
    emit("select-list", data.resourceKind, data.listParams);
  else if (data.row) emit("select-node", data.row, nodeQualifier(data));
  if (!node.leaf) {
    expandNode(node);
    await loadChildren(node);
  }
}

watch(
  () => [props.selectedGroup, props.selectedUid] as const,
  ([group, uid]) => {
    const selected = uid ? selectedNodeKey(uid) : group;
    if (selected) selectionKeys.value = { [selected]: true };
  },
  { immediate: true },
);

watch(
  () => props.groups,
  async () => {
    resetRootNodes();
    await reloadExpanded(nodes.value);
  },
  { immediate: true },
);

async function runRefresh(): Promise<void> {
  staleRefresh = false;
  await Promise.all([
    reloadExpanded(nodes.value, { force: true, loading: false }),
    loadBadges(),
  ]);
}

watch(
  () => props.refreshKey,
  async () => {
    if (!active.value) {
      staleRefresh = true;
      return;
    }
    await runRefresh();
  },
);

onActivated(() => {
  active.value = true;
  if (staleRefresh) void runRefresh();
});
onDeactivated(() => {
  active.value = false;
});

async function loadBadges(): Promise<void> {
  for (const g of props.groups) {
    if (!g.badge?.source) continue;
    try {
      const res = await fetchDoc<{ value: number }>(
        props.connectionId,
        g.badge.source,
      );
      badges[g.key] = res.value;
    } catch {
      continue;
    }
  }
}

onMounted(loadBadges);
</script>

<template>
  <Tree
    :value="nodes"
    selection-mode="single"
    loading-mode="icon"
    class="h-full min-h-0"
    :expanded-keys="expandedKeys"
    :selection-keys="selectionKeys"
    @node-expand="onNodeExpand"
    @node-collapse="onNodeCollapse"
    @node-select="onNodeSelect"
    @node-unselect="onNodeSelect"
  >
    <template #default="{ node }">
      <span
        v-if="(node as PVNode).data?.isMore"
        class="flex w-full cursor-pointer items-center gap-1.5 text-sm font-medium text-primary-600 dark:text-primary-300"
      >
        {{ node.label }}
      </span>
      <span
        v-else
        class="flex w-full cursor-pointer items-center gap-1.5"
        :title="String(node.label ?? '')"
      >
        <AppIcon
          :icon="(node as PVNode).data.icon"
          :size="15"
          class="text-surface-400"
        />
        <span class="flex-1 truncate">{{ node.label }}</span>
        <span
          v-if="badges[node.key] !== undefined"
          class="rounded-full bg-surface-200 px-1.5 text-xs text-surface-500 dark:bg-surface-700"
          >{{ badges[node.key] }}</span
        >
      </span>
    </template>
  </Tree>
</template>
