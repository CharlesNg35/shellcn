<script setup lang="ts">
import Splitter from "primevue/splitter";
import SplitterPanel from "primevue/splitterpanel";
import type { DataSource, ResourceRef } from "../../types/projection";
import TerminalPanel from "./TerminalPanel.vue";

export type TerminalGridDirection = "horizontal" | "vertical";

export type TerminalGridLayoutNode =
  | { type: "leaf"; id: string }
  | {
      type: "split";
      id: string;
      direction: TerminalGridDirection;
      first: TerminalGridLayoutNode;
      second: TerminalGridLayoutNode;
    };

defineOptions({ name: "TerminalGridNode" });

defineProps<{
  node: TerminalGridLayoutNode;
  activePaneId: string;
  connectionId: string;
  source: DataSource;
  resource?: ResourceRef | null;
  terminalConfig: Record<string, unknown>;
}>();

const emit = defineEmits<{
  focus: [paneId: string];
}>();
</script>

<template>
  <Splitter
    v-if="node.type === 'split'"
    class="h-full min-h-0 min-w-0"
    :data-terminal-grid-split="node.direction"
    :layout="node.direction === 'vertical' ? 'vertical' : 'horizontal'"
  >
    <SplitterPanel :size="50" :min-size="15" class="min-h-0 min-w-0">
      <TerminalGridNode
        :node="node.first"
        :active-pane-id="activePaneId"
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :terminal-config="terminalConfig"
        @focus="emit('focus', $event)"
      />
    </SplitterPanel>
    <SplitterPanel :size="50" :min-size="15" class="min-h-0 min-w-0">
      <TerminalGridNode
        :node="node.second"
        :active-pane-id="activePaneId"
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :terminal-config="terminalConfig"
        @focus="emit('focus', $event)"
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
    />
  </section>
</template>
