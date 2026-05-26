<script setup lang="ts">
import { ref } from "vue";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const cpu = ref<number | null>(null);
const mem = ref<number | null>(null);
const cpuHistory = ref<number[]>([]);
const memHistory = ref<number[]>([]);
const reconnecting = ref(false);

function push(history: typeof cpuHistory, value: number): void {
  history.value.push(Math.max(0, Math.min(100, value)));
  if (history.value.length > 60) history.value.shift();
}

function onFrame(frame: string): void {
  try {
    const m = JSON.parse(frame) as { cpu?: number; mem?: number };
    if (typeof m.cpu === "number") {
      cpu.value = m.cpu;
      push(cpuHistory, m.cpu);
    }
    if (typeof m.mem === "number") {
      mem.value = m.mem;
      push(memHistory, m.mem);
    }
  } catch {
    /* ignore */
  }
}

const { status, error, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  onFrame,
);

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await reconnect();
  } finally {
    reconnecting.value = false;
  }
}

const metrics = [
  { label: "CPU", value: cpu, history: cpuHistory, color: "bg-primary-500" },
  { label: "Memory", value: mem, history: memHistory, color: "bg-emerald-500" },
];
</script>

<template>
  <div class="flex h-full flex-col">
    <StreamStatusBar
      :status="status"
      :error="error"
      :reconnecting="reconnecting"
      can-reconnect
      @reconnect="onReconnect"
    />
    <div class="grid gap-4 p-6 sm:grid-cols-2">
      <div
        v-for="m in metrics"
        :key="m.label"
        class="rounded-lg border border-surface-200 p-4 dark:border-surface-800"
      >
        <p class="text-sm text-surface-400">{{ m.label }}</p>
        <p class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
          {{ m.value.value === null ? "—" : `${m.value.value}%` }}
        </p>
        <div
          class="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-200 dark:bg-surface-700"
        >
          <div
            class="h-full transition-all"
            :class="m.color"
            :style="{ width: `${m.value.value ?? 0}%` }"
          />
        </div>
        <div class="mt-4 flex h-16 items-end gap-0.5">
          <span
            v-for="(point, i) in m.history.value"
            :key="i"
            class="min-w-1 flex-1 rounded-t bg-surface-300 dark:bg-surface-700"
            :style="{ height: `${Math.max(point, 2)}%` }"
          />
        </div>
      </div>
    </div>
  </div>
</template>
