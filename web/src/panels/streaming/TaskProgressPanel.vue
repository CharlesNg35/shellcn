<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import ProgressBar from "primevue/progressbar";
import VirtualScroller from "primevue/virtualscroller";
import { runAction } from "@/api/dataSource";
import { useStream } from "@/composables/useStream";
import type { PanelProps } from "../core/types";
import type { TaskProgressPanelConfig } from "@/types/projection";
import StreamStatusBar from "./StreamStatusBar.vue";
import AppIcon from "@/components/AppIcon.vue";
import PanelLoader from "@/components/PanelLoader.vue";
import { useLogBuffer, type LogLine } from "./useLogBuffer";

const props = defineProps<PanelProps>();

const MAX = 1000;
// Wrapped lines have no fixed height, so virtualization past this threshold
// only applies to the single-line mode.
const VIRTUAL_THRESHOLD = 200;
const LINE_HEIGHT = 18;
const wrap = ref(true);

const cfg = computed(
  () => (props.config as TaskProgressPanelConfig | undefined) ?? {},
);
const { lines, append: appendLine } = useLogBuffer(MAX);
const taskStatus = ref("Running");
const percent = ref<number | null>(null);
const busy = ref(false);
const reconnecting = ref(false);

function append(frame: string): void {
  try {
    const parsed = JSON.parse(frame) as {
      status?: string;
      message?: string;
      line?: string;
      progress?: number;
      percent?: number;
      error?: string;
    };
    if (parsed.status) taskStatus.value = parsed.status;
    if (typeof parsed.percent === "number") percent.value = parsed.percent;
    else if (typeof parsed.progress === "number")
      percent.value = parsed.progress;
    const line = parsed.line ?? parsed.message ?? parsed.error;
    if (line) appendLine(line);
  } catch {
    appendLine(frame);
  }
}

const { status, error, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource, record: props.record },
  append,
);

const progressMode = computed(() =>
  percent.value == null ? "indeterminate" : "determinate",
);
const progressValue = computed(() =>
  percent.value == null ? undefined : Math.max(0, Math.min(100, percent.value)),
);
const showInitialLoader = computed(
  () => !lines.value.length && status.value === "connecting",
);
const virtualized = computed(
  () => !wrap.value && lines.value.length > VIRTUAL_THRESHOLD,
);
const emptyText = computed(() =>
  status.value === "open" ? "No task output yet." : "No task output received.",
);

async function runRoute(routeId: string | undefined): Promise<void> {
  if (!routeId) return;
  busy.value = true;
  try {
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource, record: props.record },
      {},
      {},
      "POST",
    );
  } finally {
    busy.value = false;
  }
}

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await reconnect();
  } finally {
    reconnecting.value = false;
  }
}
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
      class="space-y-3 border-b border-surface-200 px-4 py-3 dark:border-surface-800"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="min-w-0">
          <h2
            class="truncate text-sm font-semibold text-surface-900 dark:text-surface-0"
          >
            {{ cfg.title || "Task progress" }}
          </h2>
          <p class="text-xs text-surface-500 dark:text-surface-400">
            {{ taskStatus }}
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <Button
            v-if="cfg.retryRouteId"
            type="button"
            severity="secondary"
            :disabled="busy"
            @click="runRoute(cfg.retryRouteId)"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'rotate-cw' }"
              :size="14"
            />
            Retry
          </Button>
          <Button
            v-if="cfg.cancelRouteId"
            type="button"
            severity="danger"
            :disabled="busy"
            @click="runRoute(cfg.cancelRouteId)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'square' }" :size="14" />
            Cancel
          </Button>
          <Button
            type="button"
            severity="secondary"
            :label="wrap ? 'Wrap' : 'No wrap'"
            :aria-pressed="wrap"
            @click="wrap = !wrap"
          />
        </div>
      </div>
      <ProgressBar
        class="h-2"
        :mode="progressMode"
        :value="progressValue"
        :show-value="percent != null"
        :aria-label="
          progressValue == null
            ? 'Task in progress'
            : `Task progress ${progressValue}%`
        "
      />
    </div>

    <div
      class="min-h-0 flex-1 p-3 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
      :class="virtualized ? 'overflow-hidden' : 'overflow-auto'"
    >
      <VirtualScroller
        :key="virtualized ? 'virtual' : 'plain'"
        :items="lines"
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
                wrap ? 'wrap-break-word whitespace-pre-wrap' : 'whitespace-pre'
              "
              :style="virtualized ? { height: `${LINE_HEIGHT}px` } : undefined"
            >
              {{ line.text }}
            </div>
          </div>
        </template>
      </VirtualScroller>
      <PanelLoader v-if="showInitialLoader" />
      <div v-else-if="!lines.length" class="text-surface-500">
        {{ emptyText }}
      </div>
    </div>
  </div>
</template>
