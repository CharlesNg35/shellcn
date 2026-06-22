<script setup lang="ts">
import { computed, nextTick, onActivated, reactive, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import { useStream } from "@/composables/useStream";
import PanelLoader from "@/components/PanelLoader.vue";
import type { DataSource, LogStreamConfig } from "@/types/projection";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useStreamControls } from "../shared/useStreamControls";

const props = defineProps<PanelProps>();

const MAX = 1000;
const lines = ref<string[]>([]);
const follow = ref(true);
const wrap = ref(true);
const filterText = ref("");
const viewport = ref<HTMLElement | null>(null);
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
  lines.value.push(text);
  if (lines.value.length > MAX) lines.value.splice(0, lines.value.length - MAX);
  void nextTick(scrollToBottom);
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
  lines.value = [];
  void onReconnect();
}

const visibleLines = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return lines.value;
  return lines.value.filter((line) => line.toLowerCase().includes(q));
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
      class="flex items-center gap-2 border-b border-surface-200 bg-surface-0 px-3 py-2 dark:border-surface-800 dark:bg-surface-950"
    >
      <template v-for="ctrl in controls" :key="ctrl.param">
        <div v-if="controlVisible(ctrl.param)" class="w-36 shrink-0">
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
          @click="lines = []"
        />
        <Button
          as="a"
          size="small"
          severity="secondary"
          :href="downloadHref"
          download="logs.txt"
          label="Download"
        />
      </div>
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
        :class="wrap ? 'whitespace-pre-wrap' : 'whitespace-pre'"
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
