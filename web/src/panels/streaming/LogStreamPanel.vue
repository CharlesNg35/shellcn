<script setup lang="ts">
import {
  computed,
  nextTick,
  onActivated,
  onUnmounted,
  reactive,
  ref,
} from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import VirtualScroller from "primevue/virtualscroller";
import { useStream } from "@/composables/useStream";
import PanelLoader from "@/components/PanelLoader.vue";
import type { DataSource, LogStreamConfig } from "@/types/projection";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useStreamControls } from "../shared/useStreamControls";
import { useLogBuffer, type LogLine } from "./useLogBuffer";

const props = defineProps<PanelProps>();

const MAX = 1000;
// Wrapped lines have no fixed height, so virtualization past this threshold
// only applies to the single-line mode.
const VIRTUAL_THRESHOLD = 200;
const LINE_HEIGHT = 18;
const follow = ref(true);
const wrap = ref(true);
const filterText = ref("");
const viewport = ref<HTMLElement | null>(null);
const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null);
const reconnecting = ref(false);

const cfg = computed(() => props.config as LogStreamConfig | undefined);
const controls = computed(() => cfg.value?.controls ?? []);
const previous = ref(false);
const {
  values: controlValues,
  options: controlOptions,
  load: loadControls,
  visible: controlVisible,
  applyTo: applyControls,
} = useStreamControls(props.connectionId, controls, {
  resource: props.resource,
  record: props.record,
});

// A reactive copy of the source: changing a control mutates its params and
// reconnects, so the stream re-parameterizes (e.g. filter logs to one container).
const liveSource = reactive<DataSource>({
  routeId: props.source?.routeId ?? "",
  method: props.source?.method,
  params: { ...(props.source?.params ?? {}) },
});
const streamSource = props.source ? liveSource : undefined;

function scrollToBottom(): void {
  if (!follow.value) return;
  if (virtualized.value) {
    scroller.value?.scrollToIndex(visibleLines.value.length - 1);
    return;
  }
  if (viewport.value) {
    viewport.value.scrollTop = viewport.value.scrollHeight;
  }
}

const {
  lines,
  append: appendLine,
  clear: clearLines,
} = useLogBuffer(MAX, () => void nextTick(scrollToBottom));

function append(frame: string): void {
  let text = frame;
  try {
    const parsed = JSON.parse(frame) as { ts?: string; line?: string };
    if (parsed.line) text = `${parsed.ts ? `${parsed.ts} ` : ""}${parsed.line}`;
  } catch {
    /* plain text frame */
  }
  appendLine(text);
}

const { status, error, reconnect } = useStream(
  props.connectionId,
  streamSource,
  { resource: props.resource, record: props.record },
  append,
);

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await reconnect();
  } finally {
    reconnecting.value = false;
  }
}

// Re-stream with the current control selection. Clears the buffer so lines from the
// previous selection don't interleave with the new one.
function restream(): void {
  if (!props.source) return;
  applyControls(liveSource);
  if (cfg.value?.allowPrevious) {
    liveSource.params = {
      ...liveSource.params,
      previous: previous.value ? "true" : "false",
    };
  }
  clearLines();
  void onReconnect();
}

const visibleLines = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return lines.value;
  return lines.value.filter((line) => line.text.toLowerCase().includes(q));
});
const virtualized = computed(
  () => !wrap.value && visibleLines.value.length > VIRTUAL_THRESHOLD,
);
const hasLines = computed(() => lines.value.length > 0);
const showInitialLoader = computed(
  () => !hasLines.value && status.value === "connecting",
);
const emptyText = computed(() =>
  status.value === "open" ? "No log frames yet." : "No log frames received.",
);

// Built on demand: serializing the whole buffer on every append would undo the
// batching. The object URL is released once the download has been handed off.
let downloadUrl = "";
function releaseDownload(): void {
  if (!downloadUrl) return;
  URL.revokeObjectURL(downloadUrl);
  downloadUrl = "";
}

function download(): void {
  releaseDownload();
  const blob = new Blob([lines.value.map((line) => line.text).join("\n")], {
    type: "text/plain;charset=utf-8",
  });
  downloadUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = downloadUrl;
  link.download = "logs.txt";
  link.rel = "noopener";
  link.click();
  setTimeout(releaseDownload, 0);
}

void loadControls();
onActivated(() => void nextTick(scrollToBottom));
onUnmounted(releaseDownload);
</script>

<template>
  <div class="flex h-full flex-col bg-surface-0 dark:bg-surface-950">
    <StreamStatusBar
      :status="status"
      :error="error"
      :reconnecting="reconnecting"
      can-reconnect
      @reconnect="onReconnect"
    />
    <div
      class="flex flex-wrap items-center gap-2 border-b border-surface-200 bg-surface-0 px-3 py-2 dark:border-surface-800 dark:bg-surface-950"
    >
      <template v-for="ctrl in controls" :key="ctrl.param">
        <div v-if="controlVisible(ctrl.param)" class="w-36 max-w-full min-w-0">
          <Select
            v-model="controlValues[ctrl.param]"
            :options="controlOptions[ctrl.param] ?? []"
            option-label="label"
            option-value="value"
            :placeholder="ctrl.label"
            :aria-label="ctrl.label"
            size="small"
            @change="restream"
          />
        </div>
      </template>
      <div class="min-w-0 flex-1">
        <InputText
          v-model="filterText"
          size="small"
          placeholder="Filter logs"
          aria-label="Filter logs"
        />
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <Button
          type="button"
          size="small"
          severity="secondary"
          :label="follow ? 'Following' : 'Follow'"
          :aria-pressed="follow"
          @click="follow = !follow"
        />
        <Button
          type="button"
          size="small"
          severity="secondary"
          :label="wrap ? 'Wrap' : 'No wrap'"
          :aria-pressed="wrap"
          @click="wrap = !wrap"
        />
        <Button
          v-if="cfg?.allowPrevious"
          type="button"
          size="small"
          severity="secondary"
          :label="previous ? 'Previous (on)' : 'Previous'"
          :aria-pressed="previous"
          title="Show logs from the previous (crashed) container instance"
          @click="
            previous = !previous;
            restream();
          "
        />
        <Button
          type="button"
          size="small"
          severity="secondary"
          label="Clear"
          @click="clearLines"
        />
        <Button
          type="button"
          size="small"
          severity="secondary"
          label="Download"
          @click="download"
        />
      </div>
    </div>
    <div
      ref="viewport"
      data-test="log-viewport"
      role="log"
      aria-live="polite"
      class="min-h-0 flex-1 p-3 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
      :class="virtualized ? 'overflow-hidden' : 'overflow-auto'"
    >
      <VirtualScroller
        ref="scroller"
        :items="visibleLines"
        :item-size="LINE_HEIGHT"
        :disabled="!virtualized"
        scroll-height="100%"
        class="h-full"
      >
        <template #content="{ items, styleClass, contentRef, contentStyle }">
          <div :ref="contentRef" :class="styleClass" :style="contentStyle">
            <div
              v-for="line in items as LogLine[]"
              :key="line.id"
              :class="
                wrap ? 'wrap-anywhere whitespace-pre-wrap' : 'whitespace-pre'
              "
              :style="virtualized ? { height: `${LINE_HEIGHT}px` } : undefined"
            >
              {{ line.text }}
            </div>
          </div>
        </template>
      </VirtualScroller>
      <PanelLoader v-if="showInitialLoader" />
      <div v-else-if="!hasLines" class="text-surface-500">
        {{ emptyText }}
      </div>
      <div v-else-if="!visibleLines.length" class="text-surface-500">
        No matching log lines.
      </div>
    </div>
  </div>
</template>
