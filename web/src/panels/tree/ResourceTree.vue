<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue";
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

interface NodeData {
  isGroup?: boolean;
  icon?: Icon;
  ref?: ResourceIdentity;
  row?: Row;
  source?: DataSource;
  resourceKind?: string;
  listParams?: Record<string, string>;
  groupLabel?: string;
  parentPath?: string[];
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
    const page = await fetchPage<TreeNode>(props.connectionId, data.source);
    const groupLabel = data.isGroup
      ? String(node.label ?? "")
      : data.groupLabel;
    const childPath = data.isGroup
      ? []
      : [...(data.parentPath ?? []), String(node.label ?? "")];
    node.children = page.items.map((n) => toNode(n, childPath, groupLabel));
  } finally {
    if (showLoading) node.loading = false;
  }
}

async function reloadExpanded(
  nodesToReload: PVNode[],
  options: LoadChildrenOptions = {},
): Promise<void> {
  for (const node of nodesToReload) {
    if (!expandedKeys.value[String(node.key)]) continue;
    await loadChildren(node, options);
    if (node.children?.length) await reloadExpanded(node.children, options);
  }
}

function expandNode(node: PVNode): void {
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

watch(
  () => props.refreshKey,
  async () => {
    await Promise.all([
      reloadExpanded(nodes.value, { force: true, loading: false }),
      loadBadges(),
    ]);
  },
);

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
