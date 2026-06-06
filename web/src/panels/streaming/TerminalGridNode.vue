<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";
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
  paneTitles: Record<string, string>;
  canSplit: boolean;
  canClose: boolean;
}>();

const emit = defineEmits<{
  focus: [paneId: string];
  split: [paneId: string, direction: TerminalGridDirection | "auto"];
  close: [paneId: string];
}>();
</script>

<template>
  <div
    v-if="node.type === 'split'"
    class="flex h-full min-h-0 min-w-0 gap-1 bg-surface-200 dark:bg-surface-900"
    :class="node.direction === 'horizontal' ? 'flex-row' : 'flex-col'"
  >
    <TerminalGridNode
      :node="node.first"
      :active-pane-id="activePaneId"
      :connection-id="connectionId"
      :source="source"
      :resource="resource"
      :terminal-config="terminalConfig"
      :pane-titles="paneTitles"
      :can-split="canSplit"
      :can-close="canClose"
      @focus="emit('focus', $event)"
      @split="(paneId, direction) => emit('split', paneId, direction)"
      @close="emit('close', $event)"
    />
    <TerminalGridNode
      :node="node.second"
      :active-pane-id="activePaneId"
      :connection-id="connectionId"
      :source="source"
      :resource="resource"
      :terminal-config="terminalConfig"
      :pane-titles="paneTitles"
      :can-split="canSplit"
      :can-close="canClose"
      @focus="emit('focus', $event)"
      @split="(paneId, direction) => emit('split', paneId, direction)"
      @close="emit('close', $event)"
    />
  </div>

  <section
    v-else
    data-terminal-grid-pane
    :data-pane-id="node.id"
    class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden border bg-surface-0 dark:bg-surface-950"
    :class="
      node.id === activePaneId
        ? 'border-primary-500'
        : 'border-surface-200 dark:border-surface-800'
    "
    @pointerdown="emit('focus', node.id)"
  >
    <div
      class="flex min-h-8 items-center gap-1 border-b border-surface-200 bg-surface-50 px-2 text-xs dark:border-surface-800 dark:bg-surface-900"
    >
      <span class="min-w-0 flex-1 truncate font-medium">
        {{ paneTitles[node.id] ?? "Terminal" }}
      </span>
      <Button
        type="button"
        text
        rounded
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        title="Split right"
        aria-label="Split pane right"
        :pt="{ root: 'h-6 w-6 p-0' }"
        @click.stop="emit('split', node.id, 'horizontal')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'separator-vertical' }"
          :size="13"
        />
      </Button>
      <Button
        type="button"
        text
        rounded
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        title="Split down"
        aria-label="Split pane down"
        :pt="{ root: 'h-6 w-6 p-0' }"
        @click.stop="emit('split', node.id, 'vertical')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'separator-horizontal' }"
          :size="13"
        />
      </Button>
      <Button
        type="button"
        text
        rounded
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        title="Auto split"
        aria-label="Auto split pane"
        :pt="{ root: 'h-6 w-6 p-0' }"
        @click.stop="emit('split', node.id, 'auto')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'layout-dashboard' }"
          :size="13"
        />
      </Button>
      <Button
        type="button"
        text
        rounded
        size="small"
        severity="secondary"
        :disabled="!canClose"
        title="Close pane"
        aria-label="Close pane"
        :pt="{ root: 'h-6 w-6 p-0' }"
        @click.stop="emit('close', node.id)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="13" />
      </Button>
    </div>
    <div class="min-h-0 flex-1">
      <TerminalPanel
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :config="terminalConfig"
        :recording="null"
        :recording-enabled="false"
        :stream-key-suffix="node.id"
      />
    </div>
  </section>
</template>
