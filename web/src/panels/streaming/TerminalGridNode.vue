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
import Splitter from "primevue/splitter";
import type { SplitterResizeEndEvent } from "primevue/splitter";
import SplitterPanel from "primevue/splitterpanel";
import type { DataSource, ResourceRef } from "@/types/projection";
import type { ChannelStatus } from "@/stores/streamChannels";
import TerminalPanel from "./TerminalPanel.vue";

defineOptions({ name: "TerminalGridNode" });

const props = defineProps<{
  node: TerminalGridLayoutNode;
  activePaneId: string;
  connectionId: string;
  source: DataSource;
  resource?: ResourceRef | null;
  terminalConfig: Record<string, unknown>;
  splitSizes: Record<string, number[]>;
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

function structureKey(node: TerminalGridLayoutNode): string {
  if (node.type === "leaf") return node.id;
  return `${node.id}:${node.children.map((child) => structureKey(child)).join("|")}`;
}
</script>

<template>
  <Splitter
    v-if="node.type === 'split'"
    :key="structureKey(node)"
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
      :min-size="12"
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
        :terminal-config="terminalConfig"
        :split-sizes="splitSizes"
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
    class="relative h-full min-h-0 min-w-0 overflow-hidden bg-surface-0 ring-inset dark:bg-surface-950"
    :class="
      node.id === activePaneId
        ? 'ring-2 ring-primary-500'
        : 'ring-1 ring-surface-200 dark:ring-surface-800'
    "
    @pointerdown="emit('focus', node.id)"
  >
    <TerminalPanel
      :connection-id="connectionId"
      :source="source"
      :resource="resource"
      :config="terminalConfig"
      :recording="null"
      :recording-enabled="false"
      :stream-key-suffix="node.id"
      @stream-status-change="emit('streamStatusChange', node.id, $event)"
    />
  </section>
</template>
