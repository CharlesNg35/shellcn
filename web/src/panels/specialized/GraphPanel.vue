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
import SkeletonList from "../../components/SkeletonList.vue";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import RecordNode from "./RecordNode.vue";
import { buildGraph, type GraphNode, type GraphPayload } from "./graphLayout";

const props = defineProps<PanelProps>();

const loading = ref(false);
const error = ref<string | null>(null);
const payload = ref<GraphPayload>({});
const selectedId = ref<string | null>(null);
const graphConfig = computed(
  () => props.config as GraphPanelConfig | undefined,
);

const graph = computed(() => buildGraph(payload.value));

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
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
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
          :loading="loading"
        />
        Refresh
      </Button>
    </div>

    <div class="min-h-0 flex-1">
      <SkeletonList v-if="loading" />
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
