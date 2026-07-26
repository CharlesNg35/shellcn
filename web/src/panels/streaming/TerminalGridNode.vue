<script lang="ts">
export const TerminalGridDirection = {
  Horizontal: "horizontal",
  Vertical: "vertical",
} as const;
export type TerminalGridDirection =
  (typeof TerminalGridDirection)[keyof typeof TerminalGridDirection];

export type TerminalGridLayoutNode =
  | { type: "leaf"; id: string }
  | {
      type: "split";
      id: string;
      direction: TerminalGridDirection;
      children: TerminalGridLayoutNode[];
    };
</script>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { ComponentPublicInstance } from "vue";
import { useElementSize } from "@vueuse/core";
import Splitter from "primevue/splitter";
import type { SplitterResizeEndEvent } from "primevue/splitter";
import SplitterPanel from "primevue/splitterpanel";
import type { DataSource, ResourceIdentity, Row } from "@/types/projection";
import type { ChannelStatus } from "@/stores/streamChannels";
import TerminalPanel from "./TerminalPanel.vue";

defineOptions({ name: "TerminalGridNode" });

const props = defineProps<{
  node: TerminalGridLayoutNode;
  activePaneId: string;
  connectionId: string;
  source: DataSource;
  resource?: ResourceIdentity | null;
  record?: Row | null;
  terminalConfig: Record<string, unknown>;
  splitSizes: Record<string, number[]>;
  paneTitles: Record<string, string>;
}>();

const emit = defineEmits<{
  focus: [paneId: string];
  resize: [splitId: string, sizes: number[]];
  streamStatusChange: [paneId: string, status: ChannelStatus];
}>();

function evenSizes(count: number): number[] {
  const size = Number((100 / count).toFixed(4));
  const sizes = Array.from({ length: count }, () => size);
  sizes[count - 1] = Number((100 - size * (count - 1)).toFixed(4));
  return sizes;
}

function savedSizes(splitId: string, count: number): number[] {
  const sizes = props.splitSizes[splitId];
  if (!sizes || sizes.length !== count) return evenSizes(count);
  const total = sizes.reduce((sum, size) => sum + size, 0);
  if (total <= 0) return evenSizes(count);
  return sizes.map((size) => Number(((size / total) * 100).toFixed(4)));
}

function panelSize(splitId: string, index: number, count: number): number {
  return savedSizes(splitId, count)[index] ?? 100 / count;
}

function resizeSizes(event: SplitterResizeEndEvent, count: number): number[] {
  return event.sizes.length === count ? event.sizes : evenSizes(count);
}

const splitter = ref<ComponentPublicInstance | null>(null);
const { width, height } = useElementSize(splitter);

// `min-size` is a percentage, so a fixed floor (20rem across, 10rem down) has to be
// re-expressed against the measured splitter; without it panes shrink to a few
// unusable terminal columns. The per-pane cap keeps every child satisfiable.
const minPaneSize = computed(() => {
  if (props.node.type !== "split") return 12;
  const vertical = props.node.direction === "vertical";
  const extent = vertical ? height.value : width.value;
  if (extent <= 0) return 12;
  const floorPx = vertical ? 160 : 320;
  const cap = Math.floor(80 / props.node.children.length);
  return Math.max(12, Math.min(cap, Math.round((floorPx / extent) * 100)));
});

function structureKey(node: TerminalGridLayoutNode): string {
  if (node.type === "leaf") return node.id;
  return `${node.id}:${node.children.map((child) => structureKey(child)).join("|")}`;
}
</script>

<template>
  <Splitter
    v-if="node.type === 'split'"
    :key="structureKey(node)"
    ref="splitter"
    class="h-full min-h-0 min-w-0"
    :data-terminal-grid-split="node.direction"
    :layout="node.direction === 'vertical' ? 'vertical' : 'horizontal'"
    @resizeend="
      emit('resize', node.id, resizeSizes($event, node.children.length))
    "
  >
    <SplitterPanel
      v-for="(child, index) in node.children"
      :key="child.id"
      :size="panelSize(node.id, index, node.children.length)"
      :min-size="minPaneSize"
      class="min-h-0 min-w-0"
      :data-terminal-grid-panel-size="
        panelSize(node.id, index, node.children.length)
      "
    >
      <TerminalGridNode
        :node="child"
        :active-pane-id="activePaneId"
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :record="record"
        :terminal-config="terminalConfig"
        :split-sizes="splitSizes"
        :pane-titles="paneTitles"
        @focus="emit('focus', $event)"
        @resize="(splitId, sizes) => emit('resize', splitId, sizes)"
        @stream-status-change="
          (paneId, status) => emit('streamStatusChange', paneId, status)
        "
      />
    </SplitterPanel>
  </Splitter>

  <section
    v-else
    data-terminal-grid-pane
    :data-pane-id="node.id"
    role="region"
    :aria-label="`${paneTitles[node.id] ?? 'Terminal'}${
      node.id === activePaneId ? ' (active)' : ''
    }`"
    class="relative h-full min-h-0 min-w-0 overflow-hidden bg-surface-0 ring-inset dark:bg-surface-950"
    :class="
      node.id === activePaneId
        ? 'ring-2 ring-primary-500'
        : 'ring-1 ring-surface-200 dark:ring-surface-800'
    "
    @focusin="emit('focus', node.id)"
    @pointerdown="emit('focus', node.id)"
  >
    <TerminalPanel
      :connection-id="connectionId"
      :source="source"
      :resource="resource"
      :record="record"
      :config="terminalConfig"
      :recording="null"
      :recording-enabled="false"
      :stream-key-suffix="node.id"
      @stream-status-change="emit('streamStatusChange', node.id, $event)"
    />
  </section>
</template>
