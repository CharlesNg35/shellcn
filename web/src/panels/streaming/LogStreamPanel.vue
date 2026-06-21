<script setup lang="ts">
import { computed, nextTick, onActivated, reactive, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import { useStream } from "@/composables/useStream";
import { fetchPage } from "@/api/dataSource";
import PanelLoader from "@/components/PanelLoader.vue";
import type { DataSource, LogStreamConfig, Option } from "@/types/projection";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const MAX = 1000;
const lines = ref<string[]>([]);
const pausedBuffer = ref<string[]>([]);
const paused = ref(false);
const follow = ref(true);
const filterText = ref("");
const viewport = ref<HTMLElement | null>(null);
const reconnecting = ref(false);

const cfg = computed(() => props.config as LogStreamConfig | undefined);
const controls = computed(() => cfg.value?.controls ?? []);
const controlValues = reactive<Record<string, string>>({});
const controlOptions = ref<Record<string, Option[]>>({});
const previous = ref(false);

// A reactive copy of the source: changing a control mutates its params and
// reconnects, so the stream re-parameterizes (e.g. filter logs to one container).
const liveSource = reactive<DataSource>({
  routeId: props.source?.routeId ?? "",
  method: props.source?.method,
  params: { ...(props.source?.params ?? {}) },
});
const streamSource = props.source ? liveSource : undefined;

function scrollToBottom(): void {
  if (viewport.value && follow.value) {
    viewport.value.scrollTop = viewport.value.scrollHeight;
  }
}

function append(frame: string): void {
  let text = frame;
  try {
    const parsed = JSON.parse(frame) as { ts?: string; line?: string };
    if (parsed.line) text = `${parsed.ts ? `${parsed.ts} ` : ""}${parsed.line}`;
  } catch {
    /* plain text frame */
  }
  if (paused.value) {
    pausedBuffer.value.push(text);
    if (pausedBuffer.value.length > MAX) {
      pausedBuffer.value.splice(0, pausedBuffer.value.length - MAX);
    }
    return;
  }
  appendLine(text);
}

function appendLine(text: string): void {
  lines.value.push(text);
  if (lines.value.length > MAX) lines.value.splice(0, lines.value.length - MAX);
  void nextTick(scrollToBottom);
}

function togglePaused(): void {
  paused.value = !paused.value;
  if (paused.value) return;
  for (const line of pausedBuffer.value) appendLine(line);
  pausedBuffer.value = [];
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
  for (const ctrl of controls.value) {
    liveSource.params = {
      ...liveSource.params,
      [ctrl.param]: controlValues[ctrl.param] ?? "",
    };
  }
  if (cfg.value?.allowPrevious) {
    liveSource.params = {
      ...liveSource.params,
      previous: previous.value ? "true" : "false",
    };
  }
  lines.value = [];
  pausedBuffer.value = [];
  void onReconnect();
}

async function loadControls(): Promise<void> {
  for (const ctrl of controls.value) {
    if (!ctrl.optionsSource) continue;
    try {
      const page = await fetchPage<Option>(
        props.connectionId,
        ctrl.optionsSource,
        { resource: props.resource, record: props.record },
        { limit: 200 },
      );
      controlOptions.value = {
        ...controlOptions.value,
        [ctrl.param]: page.items,
      };
      if (controlValues[ctrl.param] === undefined && page.items.length) {
        controlValues[ctrl.param] = String(page.items[0].value);
      }
    } catch {
      controlOptions.value = { ...controlOptions.value, [ctrl.param]: [] };
    }
  }
}

function controlVisible(param: string): boolean {
  return (controlOptions.value[param]?.length ?? 0) > 1;
}

const visibleLines = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return lines.value;
  return lines.value.filter((line) => line.toLowerCase().includes(q));
});
const pauseLabel = computed(() => {
  if (!paused.value) return "Pause";
  return pausedBuffer.value.length
    ? `Resume (${pausedBuffer.value.length})`
    : "Resume";
});
const hasLines = computed(() => lines.value.length > 0);
const showInitialLoader = computed(
  () => !hasLines.value && status.value === "connecting",
);
const emptyText = computed(() =>
  status.value === "open" ? "No log frames yet." : "No log frames received.",
);

const downloadHref = computed(
  () =>
    `data:text/plain;charset=utf-8,${encodeURIComponent(lines.value.join("\n"))}`,
);

void loadControls();
onActivated(() => void nextTick(scrollToBottom));
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
        <Select
          v-if="controlVisible(ctrl.param)"
          v-model="controlValues[ctrl.param]"
          :options="controlOptions[ctrl.param] ?? []"
          option-label="label"
          option-value="value"
          :placeholder="ctrl.label"
          :aria-label="ctrl.label"
          class="w-48"
          @change="restream"
        />
      </template>
      <InputText
        v-model="filterText"
        placeholder="Filter logs"
        aria-label="Filter logs"
        class="max-w-64"
      />
      <Button
        type="button"
        severity="secondary"
        :label="pauseLabel"
        :aria-pressed="paused"
        @click="togglePaused"
      />
      <Button
        type="button"
        severity="secondary"
        :label="follow ? 'Following' : 'Follow'"
        :aria-pressed="follow"
        @click="follow = !follow"
      />
      <Button
        v-if="cfg?.allowPrevious"
        type="button"
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
        severity="secondary"
        label="Clear"
        @click="lines = []"
      />
      <Button
        as="a"
        severity="secondary"
        :href="downloadHref"
        download="logs.txt"
        label="Download"
      />
    </div>
    <div
      ref="viewport"
      data-test="log-viewport"
      role="log"
      aria-live="polite"
      class="min-h-0 flex-1 overflow-auto p-3 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
    >
      <div
        v-for="(line, i) in visibleLines"
        :key="i"
        class="whitespace-pre-wrap"
      >
        {{ line }}
      </div>
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
