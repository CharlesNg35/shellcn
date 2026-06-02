<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import { VueFlow, type Node } from "@vue-flow/core";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import { MiniMap } from "@vue-flow/minimap";
import "@vue-flow/core/dist/style.css";
import "@vue-flow/core/dist/theme-default.css";
import "@vue-flow/controls/dist/style.css";
import "@vue-flow/minimap/dist/style.css";
import { fetchDoc } from "../../api/dataSource";
import type { GraphPanelConfig } from "../../types/projection";
import AppIcon from "../../components/AppIcon.vue";
import PanelLoader from "../../components/PanelLoader.vue";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import RecordNode from "./RecordNode.vue";
import {
  buildGraph,
  mergeGraph,
  edgeColor,
  type GraphNode,
  type GraphPayload,
} from "./graphLayout";

const MAX_FILTER_CHIPS = 12;

const props = defineProps<PanelProps>();

const loading = ref(false);
const expanding = ref(false);
const error = ref<string | null>(null);
const payload = ref<GraphPayload>({});
const selectedId = ref<string | null>(null);
const hidden = ref<Set<string>>(new Set());
const graphConfig = computed(
  () => props.config as GraphPanelConfig | undefined,
);

const edgeLabels = computed(() => {
  const labels = new Set<string>();
  for (const edge of payload.value.edges ?? []) {
    if (edge.label) labels.add(edge.label);
  }
  return [...labels].sort();
});

const showFilter = computed(
  () =>
    edgeLabels.value.length > 1 && edgeLabels.value.length <= MAX_FILTER_CHIPS,
);

const visible = computed<GraphPayload>(() => ({
  nodes: payload.value.nodes,
  edges: (payload.value.edges ?? []).filter(
    (e) => !hidden.value.has(e.label ?? ""),
  ),
}));

const graph = computed(() => buildGraph(visible.value));

const selected = computed<GraphNode | null>(
  () => payload.value.nodes?.find((n) => n.id === selectedId.value) ?? null,
);

const properties = computed(() =>
  selected.value?.properties
    ? Object.entries(selected.value.properties).map(([key, value]) => ({
        key,
        value: String(value),
      }))
    : [],
);

const canExpand = computed(() => Boolean(graphConfig.value?.expandRouteId));

function toggleLabel(label: string): void {
  const next = new Set(hidden.value);
  if (next.has(label)) {
    next.delete(label);
  } else {
    next.add(label);
  }
  hidden.value = next;
}

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  hidden.value = new Set();
  try {
    payload.value = await fetchDoc<GraphPayload>(
      props.connectionId,
      props.source,
      {
        resource: props.resource,
      },
    );
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function expand(nodeId: string): Promise<void> {
  const routeId = graphConfig.value?.expandRouteId;
  if (!routeId || expanding.value) return;
  expanding.value = true;
  try {
    const param = graphConfig.value?.expandParam || "node";
    const incoming = await fetchDoc<GraphPayload>(
      props.connectionId,
      { routeId, params: { ...props.source?.params, [param]: nodeId } },
      { resource: props.resource },
    );
    payload.value = mergeGraph(payload.value, incoming);
  } catch {
    // Best effort: a failed expansion leaves the current graph intact.
  } finally {
    expanding.value = false;
  }
}

function selectNode(event: { node: Node }): void {
  selectedId.value = event.node.id;
}

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center justify-between border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <div class="flex items-center gap-2 text-sm text-surface-500">
        <AppIcon :icon="{ type: 'lucide', value: 'workflow' }" :size="16" />
        <span>{{ graph.nodes.length }} nodes</span>
        <span>{{ graph.edges.length }} edges</span>
        <span v-if="canExpand" class="text-xs text-surface-400"
          >· double-click a node to expand</span
        >
      </div>
      <Button
        type="button"
        severity="secondary"
        :disabled="loading"
        @click="load"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="loading || expanding"
        />
        Refresh
      </Button>
    </div>

    <div
      v-if="showFilter"
      class="flex flex-wrap items-center gap-1.5 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <Button
        v-for="label in edgeLabels"
        :key="label"
        size="small"
        rounded
        severity="secondary"
        :variant="hidden.has(label) ? 'outlined' : 'text'"
        :aria-pressed="!hidden.has(label)"
        :class="{ 'opacity-50': hidden.has(label) }"
        @click="toggleLabel(label)"
      >
        <span
          class="h-2 w-2 rounded-full"
          :style="{ backgroundColor: edgeColor(label) }"
        />
        {{ label }}
      </Button>
    </div>

    <div class="min-h-0 flex-1">
      <PanelLoader v-if="loading" />
      <PanelError v-else-if="error" :message="error" retryable @retry="load" />
      <div
        v-else-if="!graph.nodes.length"
        class="flex h-full items-center justify-center p-6 text-sm text-surface-400"
      >
        No graph data.
      </div>
      <VueFlow
        v-else
        :nodes="graph.nodes"
        :edges="graph.edges"
        :fit-view-on-init="graphConfig?.fitView ?? true"
        :min-zoom="0.1"
        :nodes-connectable="false"
        class="h-full bg-surface-50 dark:bg-surface-950"
        @node-click="selectNode"
        @node-double-click="expand($event.node.id)"
      >
        <template #node-record="recordProps">
          <RecordNode
            :data="recordProps.data"
            :selected="recordProps.selected"
          />
        </template>
        <Background :gap="16" />
        <Controls />
        <MiniMap pannable zoomable />
      </VueFlow>
    </div>

    <div
      v-if="selected?.summary || properties.length"
      class="max-h-40 overflow-auto border-t border-surface-200 p-3 text-sm dark:border-surface-800"
    >
      <p class="font-semibold text-surface-900 dark:text-surface-0">
        {{ selected?.label || selected?.id }}
      </p>
      <p v-if="selected?.summary" class="mt-1 text-surface-500">
        {{ selected?.summary }}
      </p>
      <dl
        v-if="properties.length"
        class="mt-2 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1"
      >
        <template v-for="p in properties" :key="p.key">
          <dt class="text-surface-400">{{ p.key }}</dt>
          <dd class="break-all text-surface-600 dark:text-surface-300">
            {{ p.value }}
          </dd>
        </template>
      </dl>
    </div>
  </div>
</template>

<style>
.shellcn-graph-node {
  border-color: color-mix(in srgb, var(--p-primary-color) 55%, transparent);
  border-radius: 8px;
  font-size: 12px;
}
</style>
