<script setup lang="ts">
import { ref } from "vue";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

const cpu = ref<number | null>(null);
const mem = ref<number | null>(null);

function onFrame(frame: string): void {
  try {
    const m = JSON.parse(frame) as { cpu?: number; mem?: number };
    if (typeof m.cpu === "number") cpu.value = m.cpu;
    if (typeof m.mem === "number") mem.value = m.mem;
  } catch {
    /* ignore */
  }
}

const { status } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  onFrame,
);

const metrics = [
  { label: "CPU", value: cpu, color: "bg-primary-500" },
  { label: "Memory", value: mem, color: "bg-emerald-500" },
];
</script>

<template>
  <div class="flex h-full flex-col">
    <StubBanner :status="status" />
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
      </div>
    </div>
  </div>
</template>
