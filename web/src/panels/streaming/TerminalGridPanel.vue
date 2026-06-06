<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";
import PanelError from "../shared/PanelError.vue";
import type { PanelProps } from "../core/types";
import { channelKey } from "../../api/dataSource";
import { useStreamChannelsStore } from "../../stores/streamChannels";
import type {
  TerminalGridPanelConfig,
  TerminalPanelConfig,
} from "../../types/projection";
import TerminalGridNode, {
  type TerminalGridDirection,
  type TerminalGridLayoutNode,
} from "./TerminalGridNode.vue";

const props = defineProps<PanelProps>();
const streams = useStreamChannelsStore();

const cfg = computed(
  () => (props.config as TerminalGridPanelConfig | undefined) ?? {},
);
const maxPanes = computed(() =>
  Math.max(1, Math.min(12, Math.floor(cfg.value.maxPanes ?? 6))),
);
const defaultPanes = computed(() =>
  Math.max(
    1,
    Math.min(maxPanes.value, Math.floor(cfg.value.defaultPanes ?? 1)),
  ),
);
const terminalConfig = computed<Record<string, unknown> & TerminalPanelConfig>(
  () => ({
    zoom: cfg.value.zoom,
    search: cfg.value.search,
  }),
);

let seq = 0;
const layout = ref<TerminalGridLayoutNode>(leaf());
const activePaneId = ref(layout.value.id);
const root = ref<HTMLElement | null>(null);
const initialized = ref(false);

function leaf(): TerminalGridLayoutNode {
  seq += 1;
  return { type: "leaf", id: `pane-${seq}` };
}

function paneIds(node: TerminalGridLayoutNode = layout.value): string[] {
  if (node.type === "leaf") return [node.id];
  return [...paneIds(node.first), ...paneIds(node.second)];
}

const paneCount = computed(() => paneIds().length);
const canSplit = computed(() => paneCount.value < maxPanes.value);
const canClose = computed(() => paneCount.value > 1);
const paneTitles = computed(() =>
  Object.fromEntries(
    paneIds().map((id, index) => [id, `Terminal ${index + 1}`]),
  ),
);
const recordingPolicy = computed(() => props.recording?.policy ?? "disabled");
const blocksRecording = computed(() => recordingPolicy.value === "auto");
const showsRecordingNotice = computed(
  () => recordingPolicy.value === "manual" && paneCount.value > 1,
);

function splitTree(
  node: TerminalGridLayoutNode,
  paneId: string,
  direction: TerminalGridDirection,
): TerminalGridLayoutNode {
  if (node.type === "leaf") {
    if (node.id !== paneId) return node;
    const next = leaf();
    activePaneId.value = next.id;
    return {
      type: "split",
      id: `split-${next.id}`,
      direction,
      first: node,
      second: next,
    };
  }
  return {
    ...node,
    first: splitTree(node.first, paneId, direction),
    second: splitTree(node.second, paneId, direction),
  };
}

function removeLeaf(
  node: TerminalGridLayoutNode,
  paneId: string,
): TerminalGridLayoutNode | null {
  if (node.type === "leaf") return node.id === paneId ? null : node;
  const first = removeLeaf(node.first, paneId);
  const second = removeLeaf(node.second, paneId);
  if (!first) return second;
  if (!second) return first;
  return { ...node, first, second };
}

function paneDirection(paneId: string): TerminalGridDirection {
  const el = root.value?.querySelector<HTMLElement>(
    `[data-pane-id="${paneId}"]`,
  );
  if (!el) return "horizontal";
  const rect = el.getBoundingClientRect();
  return rect.width >= rect.height ? "horizontal" : "vertical";
}

function closePaneStream(paneId: string): void {
  if (!props.source) return;
  streams.close(
    `${channelKey(props.connectionId, props.source, { resource: props.resource })}:${paneId}`,
  );
}

function closeAllPaneStreams(): void {
  for (const paneId of paneIds()) closePaneStream(paneId);
}

function splitPane(
  paneId = activePaneId.value,
  direction: TerminalGridDirection | "auto" = "auto",
): void {
  if (!props.source || !canSplit.value) return;
  const resolved = direction === "auto" ? paneDirection(paneId) : direction;
  layout.value = splitTree(layout.value, paneId, resolved);
}

function closePane(paneId = activePaneId.value): void {
  if (!canClose.value) return;
  closePaneStream(paneId);
  const next = removeLeaf(layout.value, paneId);
  if (!next) return;
  layout.value = next;
  if (!paneIds().includes(activePaneId.value)) {
    activePaneId.value = paneIds()[0] ?? "";
  }
}

function resetLayout(): void {
  if (initialized.value) closeAllPaneStreams();
  seq = 0;
  layout.value = leaf();
  activePaneId.value = layout.value.id;
  for (let i = 1; i < defaultPanes.value; i += 1) {
    splitPane(activePaneId.value, i % 2 === 0 ? "vertical" : "horizontal");
  }
}

onMounted(() => {
  resetLayout();
  initialized.value = true;
});

onBeforeUnmount(closeAllPaneStreams);
</script>

<template>
  <PanelError v-if="!source" message="No terminal stream route configured." />
  <PanelError
    v-else-if="blocksRecording"
    message="Split terminal workspaces are disabled when terminal recording is mandatory."
  />
  <div v-else ref="root" class="flex h-full min-h-0 flex-col bg-surface-950">
    <div
      class="flex min-h-10 items-center gap-2 border-b border-surface-200 bg-surface-0 px-3 py-1.5 dark:border-surface-800 dark:bg-surface-950"
    >
      <span class="min-w-0 flex-1 truncate text-sm font-medium">
        Terminal workspace
      </span>
      <span
        v-if="showsRecordingNotice"
        class="hidden text-xs text-amber-600 sm:inline dark:text-amber-400"
      >
        Recording disabled for split view
      </span>
      <Button
        type="button"
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        @click="splitPane(activePaneId, 'horizontal')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'separator-vertical' }"
          :size="14"
        />
        Split right
      </Button>
      <Button
        type="button"
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        @click="splitPane(activePaneId, 'vertical')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'separator-horizontal' }"
          :size="14"
        />
        Split down
      </Button>
      <Button
        type="button"
        size="small"
        severity="secondary"
        :disabled="!canSplit"
        @click="splitPane(activePaneId, 'auto')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'layout-dashboard' }"
          :size="14"
        />
        Auto
      </Button>
      <Button
        type="button"
        size="small"
        severity="secondary"
        outlined
        @click="resetLayout"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'rotate-ccw' }" :size="14" />
        Reset
      </Button>
    </div>

    <div class="min-h-0 flex-1 bg-surface-200 p-1 dark:bg-surface-900">
      <TerminalGridNode
        v-if="initialized"
        :node="layout"
        :active-pane-id="activePaneId"
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :terminal-config="terminalConfig"
        :pane-titles="paneTitles"
        :can-split="canSplit"
        :can-close="canClose"
        @focus="activePaneId = $event"
        @split="splitPane"
        @close="closePane"
      />
    </div>
  </div>
</template>
