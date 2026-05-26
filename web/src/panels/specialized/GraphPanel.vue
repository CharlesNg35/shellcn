<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import { VueFlow, type Edge, type Node } from "@vue-flow/core";
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
import SkeletonList from "../../components/SkeletonList.vue";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";

interface GraphNode {
  id: string;
  label?: string;
  type?: string;
  group?: string;
  summary?: string;
  position?: { x: number; y: number };
  properties?: Record<string, unknown>;
}

interface GraphEdge {
  id?: string;
  source: string;
  target: string;
  label?: string;
  animated?: boolean;
}

interface GraphPayload {
  nodes?: GraphNode[];
  edges?: GraphEdge[];
}

const props = defineProps<PanelProps>();

const loading = ref(false);
const error = ref<string | null>(null);
const payload = ref<GraphPayload>({});
const selected = ref<GraphNode | null>(null);
const graphConfig = computed(
  () => props.config as GraphPanelConfig | undefined,
);

function gridPosition(i: number): { x: number; y: number } {
  const col = i % 4;
  const row = Math.floor(i / 4);
  return { x: col * 220, y: row * 140 };
}

const nodes = computed<Node[]>(() =>
  (payload.value.nodes ?? []).map((node, i) => ({
    id: node.id,
    position:
      graphConfig.value?.layout === "manual" && node.position
        ? node.position
        : (node.position ?? gridPosition(i)),
    data: { label: node.label ?? node.id },
    type:
      node.type === "input" || node.type === "output" ? node.type : "default",
    class: "shellcn-graph-node",
  })),
);

const edges = computed<Edge[]>(() =>
  (payload.value.edges ?? []).map((edge, i) => ({
    id: edge.id ?? `${edge.source}-${edge.target}-${i}`,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    animated: edge.animated,
  })),
);

const properties = computed(() =>
  selected.value?.properties
    ? Object.entries(selected.value.properties).map(([key, value]) => ({
        key,
        value,
      }))
    : [],
);

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    payload.value = await fetchDoc<GraphPayload>(
      props.connectionId,
      props.source,
      {
        resource: props.resource,
      },
    );
    selected.value = payload.value.nodes?.[0] ?? null;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function selectNode(event: { node: Node }): void {
  selected.value =
    payload.value.nodes?.find((node) => node.id === event.node.id) ?? null;
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
        <AppIcon :icon="{ type: 'name', value: 'workflow' }" :size="16" />
        <span>{{ nodes.length }} nodes</span>
        <span>{{ edges.length }} edges</span>
      </div>
      <Button
        type="button"
        severity="secondary"
        :disabled="loading"
        @click="load"
      >
        Refresh
      </Button>
    </div>

    <div class="min-h-0 flex-1">
      <SkeletonList v-if="loading" />
      <PanelError v-else-if="error" :message="error" retryable @retry="load" />
      <div
        v-else-if="!nodes.length"
        class="flex h-full items-center justify-center p-6 text-sm text-surface-400"
      >
        No graph data.
      </div>
      <div v-else class="grid h-full min-h-0 grid-cols-[minmax(0,1fr)_18rem]">
        <div
          class="min-h-0 border-r border-surface-200 dark:border-surface-800"
        >
          <VueFlow
            :nodes="nodes"
            :edges="edges"
            :fit-view-on-init="graphConfig?.fitView ?? true"
            :nodes-draggable="false"
            class="h-full bg-surface-50 dark:bg-surface-950"
            @node-click="selectNode"
          >
            <Background />
            <Controls />
            <MiniMap pannable zoomable />
          </VueFlow>
        </div>
        <aside class="min-h-0 overflow-auto p-4">
          <p v-if="!selected" class="text-sm text-surface-400">
            Select a node.
          </p>
          <template v-else>
            <p class="text-xs text-surface-400 uppercase">
              {{ selected.group || selected.type || "Node" }}
            </p>
            <h3 class="mt-1 font-semibold text-surface-900 dark:text-surface-0">
              {{ selected.label || selected.id }}
            </h3>
            <p v-if="selected.summary" class="mt-2 text-sm text-surface-500">
              {{ selected.summary }}
            </p>
            <DataTable
              v-if="properties.length"
              :value="properties"
              class="mt-4"
              scrollable
              scroll-height="16rem"
            >
              <Column field="key" header="Key" />
              <Column header="Value">
                <template #body="{ data }">
                  <span
                    class="break-all text-surface-600 dark:text-surface-300"
                  >
                    {{ String(data.value) }}
                  </span>
                </template>
              </Column>
            </DataTable>
          </template>
        </aside>
      </div>
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
