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
}

const props = defineProps<{
  connectionId: string;
  groups: TreeGroup[];
  selectedUid?: string;
  selectedGroup?: string;
}>();
const emit = defineEmits<{
  "select-group": [key: string];
  "select-node": [row: Row];
}>();

const nodes = ref<PVNode[]>([]);
const badges = reactive<Record<string, string | number>>({});
const expandedKeys = ref<Record<string, boolean>>({});
const selectionKeys = ref<Record<string, boolean>>({});

function toNode(n: TreeNode): PVNode {
  return {
    key: n.key,
    label: n.label,
    leaf: !n.childrenSource,
    data: {
      icon: n.icon,
      ref: n.ref,
      row: { ...n, ref: n.ref },
      source: n.childrenSource,
    },
  };
}

watchEffect(() => {
  nodes.value = props.groups.map((g) => ({
    key: g.key,
    label: g.label,
    leaf: false,
    data: { isGroup: true, icon: g.icon, source: g.source },
  }));
});

async function loadChildren(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  if (node.children || !data.source) return;
  node.loading = true;
  try {
    const page = await fetchPage<TreeNode>(props.connectionId, data.source);
    node.children = page.items.map(toNode);
  } finally {
    node.loading = false;
  }
}

// Single click selects AND, for a branch, expands + lazy-loads — one gesture to
// drill in (and a simpler, more predictable interaction).
async function onNodeSelect(node: PVNode): Promise<void> {
  const data = node.data as NodeData;
  selectionKeys.value = { [String(node.key)]: true };
  if (data.isGroup) emit("select-group", String(node.key));
  else if (data.row) emit("select-node", data.row);
  if (!node.leaf) {
    expandedKeys.value = { ...expandedKeys.value, [String(node.key)]: true };
    await loadChildren(node);
  }
}

watch(
  () => [props.selectedGroup, props.selectedUid] as const,
  ([group, uid]) => {
    const selected = uid ?? group;
    selectionKeys.value = selected ? { [selected]: true } : {};
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
    @update:expanded-keys="expandedKeys = $event"
    @node-expand="loadChildren"
    @node-select="onNodeSelect"
  >
    <template #default="{ node }">
      <span class="flex w-full items-center gap-1.5">
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
