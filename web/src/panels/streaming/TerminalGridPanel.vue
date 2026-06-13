<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import Button from "primevue/button";
import Tooltip from "primevue/tooltip";
import AppIcon from "@/components/AppIcon.vue";
import RecordingControls from "@/components/recordings/RecordingControls.vue";
import PanelError from "../shared/PanelError.vue";
import type { PanelProps } from "../core/types";
import { channelKey } from "@/api/dataSource";
import { useStreamChannelsStore } from "@/stores/streamChannels";
import type { ChannelStatus } from "@/stores/streamChannels";
import type {
  RecordingPolicy,
  TerminalGridPanelConfig,
  TerminalPanelConfig,
} from "@/types/projection";
import { RecordingPolicy as RecordingPolicyEnum } from "@/types/projection";
import TerminalGridNode, {
  TerminalGridDirection,
  type TerminalGridDirection as TerminalGridDirectionValue,
  type TerminalGridLayoutNode,
} from "./TerminalGridNode.vue";

const TerminalGridAutoDirection = "auto" as const;

const props = defineProps<PanelProps>();
const streams = useStreamChannelsStore();
const vTooltip = Tooltip;

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
const splitSizes = ref<Record<string, number[]>>({});
const paneStatuses = ref<Record<string, ChannelStatus>>({});
const recordingActive = ref(false);
const root = ref<HTMLElement | null>(null);
const initialized = ref(false);

function leaf(): TerminalGridLayoutNode {
  seq += 1;
  return { type: "leaf", id: `pane-${seq}` };
}

function paneIds(node: TerminalGridLayoutNode = layout.value): string[] {
  if (node.type === "leaf") return [node.id];
  return node.children.flatMap((child) => paneIds(child));
}

function splitSpecs(
  node: TerminalGridLayoutNode = layout.value,
): Record<string, number> {
  if (node.type === "leaf") return {};
  return Object.assign(
    { [node.id]: node.children.length },
    ...node.children.map((child) => splitSpecs(child)),
  );
}

const paneCount = computed(() => paneIds().length);
const singlePane = computed(() => paneCount.value === 1);
const canClose = computed(() => paneCount.value > 1);
const paneTitles = computed(() =>
  Object.fromEntries(
    paneIds().map((id, index) => [id, `Terminal ${index + 1}`]),
  ),
);
const activePaneLabel = computed(
  () => paneTitles.value[activePaneId.value] ?? "Terminal",
);
const recordingPolicy = computed<RecordingPolicy>(
  () => props.recording?.policy ?? RecordingPolicyEnum.Disabled,
);
const showRecording = computed(
  () =>
    props.source && props.recording?.policy !== RecordingPolicyEnum.Disabled,
);
const recordingBlocksLayout = computed(
  () =>
    recordingPolicy.value === RecordingPolicyEnum.Auto || recordingActive.value,
);
const canSplit = computed(
  () => paneCount.value < maxPanes.value && !recordingBlocksLayout.value,
);
const recordingDisabledReason = computed(() =>
  singlePane.value
    ? null
    : "Recording is unavailable when multiple terminal panes are open.",
);
const activeStreamStatus = computed(
  () => paneStatuses.value[activePaneId.value],
);

function buttonTooltip(value: string): { value: string; showDelay: number } {
  return { value, showDelay: 300 };
}

function splitButtonTooltip(value: string): {
  value: string;
  showDelay: number;
} {
  return {
    value: recordingBlocksLayout.value
      ? "Stop recording before splitting terminal panes."
      : value,
    showDelay: 300,
  };
}

function layoutButtonTooltip(value: string): {
  value: string;
  showDelay: number;
} {
  return {
    value: recordingBlocksLayout.value
      ? "Stop recording before changing terminal layout."
      : value,
    showDelay: 300,
  };
}

function splitTree(
  node: TerminalGridLayoutNode,
  paneId: string,
  direction: TerminalGridDirectionValue,
): TerminalGridLayoutNode {
  if (node.type === "leaf") {
    if (node.id !== paneId) return node;
    const next = leaf();
    activePaneId.value = next.id;
    return {
      type: "split",
      id: `split-${next.id}`,
      direction,
      children: [node, next],
    };
  }
  return normalizeSplit({
    ...node,
    children: node.children.map((child) => splitTree(child, paneId, direction)),
  });
}

function normalizeSplit(node: TerminalGridLayoutNode): TerminalGridLayoutNode {
  if (node.type === "leaf") return node;
  const children = node.children
    .map((child) => normalizeSplit(child))
    .flatMap((child) =>
      child.type === "split" && child.direction === node.direction
        ? child.children
        : [child],
    );
  if (children.length === 1) return children[0];
  return { ...node, children };
}

function removeLeaf(
  node: TerminalGridLayoutNode,
  paneId: string,
): TerminalGridLayoutNode | null {
  if (node.type === "leaf") return node.id === paneId ? null : node;
  const children = node.children.flatMap((child) => {
    const next = removeLeaf(child, paneId);
    return next ? [next] : [];
  });
  if (children.length === 0) return null;
  if (children.length === 1) return children[0];
  return normalizeSplit({ ...node, children });
}

function paneDirection(paneId: string): TerminalGridDirectionValue {
  const el = root.value?.querySelector<HTMLElement>(
    `[data-pane-id="${paneId}"]`,
  );
  if (!el) return TerminalGridDirection.Horizontal;
  const rect = el.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0)
    return TerminalGridDirection.Horizontal;
  const horizontalRatio = aspectRatio(rect.width / 2, rect.height);
  const verticalRatio = aspectRatio(rect.width, rect.height / 2);
  return horizontalRatio <= verticalRatio
    ? TerminalGridDirection.Horizontal
    : TerminalGridDirection.Vertical;
}

function aspectRatio(width: number, height: number): number {
  const min = Math.max(1, Math.min(width, height));
  return Math.max(width, height) / min;
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
  direction:
    | TerminalGridDirectionValue
    | typeof TerminalGridAutoDirection = TerminalGridAutoDirection,
): void {
  if (!props.source || !canSplit.value) return;
  const resolved =
    direction === TerminalGridAutoDirection ? paneDirection(paneId) : direction;
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

function resetLayout(force = false): void {
  if (!force && recordingBlocksLayout.value) return;
  if (initialized.value) closeAllPaneStreams();
  splitSizes.value = {};
  paneStatuses.value = {};
  seq = 0;
  layout.value = leaf();
  activePaneId.value = layout.value.id;
  for (let i = 1; i < defaultPanes.value; i += 1) {
    splitPane(
      activePaneId.value,
      i % 2 === 0
        ? TerminalGridDirection.Vertical
        : TerminalGridDirection.Horizontal,
    );
  }
}

function onResize(splitId: string, sizes: number[]): void {
  splitSizes.value = { ...splitSizes.value, [splitId]: sizes };
}

function onPaneStatusChange(paneId: string, status: ChannelStatus): void {
  paneStatuses.value = { ...paneStatuses.value, [paneId]: status };
}

onMounted(() => {
  resetLayout(true);
  initialized.value = true;
});

onBeforeUnmount(closeAllPaneStreams);

watch(
  layout,
  () => {
    const specs = splitSpecs();
    splitSizes.value = Object.fromEntries(
      Object.entries(splitSizes.value).filter(
        ([splitId, sizes]) => specs[splitId] === sizes.length,
      ),
    );
  },
  { deep: true },
);
</script>

<template>
  <PanelError v-if="!source" message="No terminal stream route configured." />
  <div
    v-else
    ref="root"
    class="flex h-full min-h-0 flex-col bg-surface-0 dark:bg-surface-950"
  >
    <div
      class="flex min-h-8 items-center gap-2 border-b border-surface-200 bg-surface-0 px-2 py-0.5 dark:border-surface-800 dark:bg-surface-950"
    >
      <div class="flex min-w-0 flex-1 items-center gap-2">
        <div class="truncate text-xs font-medium">{{ activePaneLabel }}</div>
        <div class="shrink-0 text-xs text-surface-500 dark:text-surface-400">
          {{ paneCount }} / {{ maxPanes }} panes
        </div>
      </div>
      <div class="flex shrink-0 items-center gap-0.5">
        <RecordingControls
          v-if="showRecording && source && recording"
          :connection-id="connectionId"
          :source="source"
          :resource="resource"
          :descriptor="recording"
          :disabled-reason="recordingDisabledReason"
          :stream-status="activeStreamStatus"
          @recording-change="recordingActive = $event"
        />
        <span
          v-if="showRecording"
          class="mx-1 h-4 w-px bg-surface-200 dark:bg-surface-800"
          aria-hidden="true"
        />
        <Button
          v-tooltip.bottom="splitButtonTooltip('Split right')"
          type="button"
          size="small"
          severity="secondary"
          text
          rounded
          class="h-7 w-7 px-0 py-0"
          :disabled="!canSplit"
          aria-label="Split active pane right"
          @click="splitPane(activePaneId, 'horizontal')"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'separator-vertical' }"
            :size="14"
          />
          <span class="sr-only">Split right</span>
        </Button>
        <Button
          v-tooltip.bottom="splitButtonTooltip('Split down')"
          type="button"
          size="small"
          severity="secondary"
          text
          rounded
          class="h-7 w-7 px-0 py-0"
          :disabled="!canSplit"
          aria-label="Split active pane down"
          @click="splitPane(activePaneId, 'vertical')"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'separator-horizontal' }"
            :size="14"
          />
          <span class="sr-only">Split down</span>
        </Button>
        <Button
          v-tooltip.bottom="splitButtonTooltip('Auto split')"
          type="button"
          size="small"
          severity="secondary"
          text
          rounded
          class="h-7 w-7 px-0 py-0"
          :disabled="!canSplit"
          aria-label="Auto split active pane"
          @click="splitPane(activePaneId, 'auto')"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'layout-dashboard' }"
            :size="14"
          />
          <span class="sr-only">Auto split</span>
        </Button>
        <Button
          v-tooltip.bottom="buttonTooltip('Close pane')"
          type="button"
          size="small"
          severity="secondary"
          text
          rounded
          class="h-7 w-7 px-0 py-0"
          aria-label="Close active pane"
          :disabled="!canClose"
          @click="closePane(activePaneId)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
          <span class="sr-only">Close pane</span>
        </Button>
        <Button
          v-tooltip.bottom="layoutButtonTooltip('Reset layout')"
          type="button"
          size="small"
          severity="secondary"
          text
          rounded
          class="h-7 w-7 px-0 py-0"
          aria-label="Reset terminal workspace"
          :disabled="recordingBlocksLayout"
          @click="resetLayout()"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'rotate-ccw' }" :size="14" />
          <span class="sr-only">Reset layout</span>
        </Button>
      </div>
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
        :split-sizes="splitSizes"
        @focus="activePaneId = $event"
        @resize="onResize"
        @stream-status-change="onPaneStatusChange"
      />
    </div>
  </div>
</template>
