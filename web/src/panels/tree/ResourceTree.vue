<script setup lang="ts">
import { onMounted, reactive, ref, watch, watchEffect } from "vue";
import Tree from "primevue/tree";
import type { TreeNode as PVNode } from "primevue/treenode";
import { fetchDoc, fetchPage } from "../../api/dataSource";
import type {
  DataSource,
  Icon,
  ResourceRef,
  Row,
  TreeGroup,
  TreeNode,
} from "../../types/projection";
import AppIcon from "../../components/AppIcon.vue";

interface NodeData {
  isGroup?: boolean;
  icon?: Icon;
  ref?: ResourceRef;
  row?: Row;
  source?: DataSource;
  resourceKind?: string;
  listParams?: Record<string, string>;
  // Tab qualifier: the intermediate ancestor labels (db / schema), or the root
  // group label when the node sits directly under it (Containers, Compose).
  groupLabel?: string;
  parentPath?: string[];
}

// Intermediate ancestors when present, else the category (root group) name.
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

watchEffect(() => {
  nodes.value = props.groups.map((g) => ({
    key: g.key,
    label: g.label,
    // A group with no children source is a leaf: it opens its destination
    // directly (a kind list, or a resource detail via ref) without expanding.
    leaf: !g.source?.routeId,
    data: {
      isGroup: true,
      icon: g.icon,
      source: g.source,
      ref: g.ref,
      row: g.ref ? { ref: g.ref } : undefined,
    },
  }));
});

async function loadChildren(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  if (node.children || !data.source?.routeId) return;
  node.loading = true;
  try {
    const page = await fetchPage<TreeNode>(props.connectionId, data.source);
    // The root group names the category; deeper nodes add their label to the path.
    const groupLabel = data.isGroup
      ? String(node.label ?? "")
      : data.groupLabel;
    const childPath = data.isGroup
      ? []
      : [...(data.parentPath ?? []), String(node.label ?? "")];
    node.children = page.items.map((n) => toNode(n, childPath, groupLabel));
  } finally {
    node.loading = false;
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

// Single click selects AND, for a branch, expands + lazy-loads — one gesture to
// drill in. PrimeVue single-selection emits node-unselect when the highlighted
// node is clicked again, but that is still an activation in this navigator.
async function onNodeSelect(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  selectionKeys.value = { [String(node.key)]: true };
  if (data.isGroup) {
    // A leaf group pointing at a specific resource opens that detail directly;
    // otherwise it opens its list (or expands, for a group with children).
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
    // Sync highlight to a group/detail selection; leave a click-driven
    // selection (e.g. a list-opening node) untouched when neither matches.
    const selected = uid ? selectedNodeKey(uid) : group;
    if (selected) selectionKeys.value = { [selected]: true };
  },
  { immediate: true },
);

onMounted(async () => {
  for (const g of props.groups) {
    if (!g.badge?.source) continue;
    try {
      const res = await fetchDoc<{ value: number }>(
        props.connectionId,
        g.badge.source,
      );
      badges[g.key] = res.value;
    } catch {
      // best-effort
    }
  }
});
</script>

<template>
  <Tree
    :value="nodes"
    selection-mode="single"
    loading-mode="icon"
    class="h-full"
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
