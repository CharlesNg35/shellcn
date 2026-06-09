<script setup lang="ts">
import { computed, nextTick, onActivated, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import { useStream } from "../../composables/useStream";
import PanelLoader from "../../components/PanelLoader.vue";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const MAX = 1000;
const lines = ref<string[]>([]);
const paused = ref(false);
const follow = ref(true);
const filterText = ref("");
const viewport = ref<HTMLElement | null>(null);
const reconnecting = ref(false);

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
  if (paused.value) return;
  lines.value.push(text);
  if (lines.value.length > MAX) lines.value.splice(0, lines.value.length - MAX);
  void nextTick(scrollToBottom);
}

const { status, error, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
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
      <InputText
        v-model="filterText"
        placeholder="Filter logs"
        class="max-w-64"
      />
      <Button
        type="button"
        severity="secondary"
        :label="paused ? 'Resume' : 'Pause'"
        @click="paused = !paused"
      />
      <Button
        type="button"
        severity="secondary"
        :label="follow ? 'Following' : 'Follow'"
        @click="follow = !follow"
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
