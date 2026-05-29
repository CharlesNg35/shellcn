<script setup lang="ts">
import { computed, ref } from "vue";
import type { ChartData, ChartOptions } from "chart.js";
import { useTheme } from "../../../composables/useTheme";
import { seriesColor } from "./chartTheme";
import { useChart } from "./useChart";

const props = withDefaults(
  defineProps<{
    label: string;
    value: number | null;
    max?: number;
    unit?: string;
    colorIndex?: number;
  }>(),
  { max: 100, unit: "", colorIndex: 0 },
);

const { isDark } = useTheme();
const color = computed(() => seriesColor(props.colorIndex));

const pct = computed(() => {
  if (props.value === null) return 0;
  const max = props.max && props.max > 0 ? props.max : 100;
  return Math.max(0, Math.min(100, (props.value / max) * 100));
});

const data = computed<ChartData>(() => ({
  labels: ["value", "rest"],
  datasets: [
    {
      data: [pct.value, 100 - pct.value],
      backgroundColor: [
        color.value,
        isDark.value ? "rgba(255,255,255,0.08)" : "rgba(0,0,0,0.06)",
      ],
      borderWidth: 0,
    },
  ],
}));

const options = computed<ChartOptions>(() => ({
  cutout: "78%",
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false }, tooltip: { enabled: false } },
  animation: { duration: 400 },
}));

const canvasEl = ref<HTMLCanvasElement | null>(null);
useChart(
  canvasEl,
  "doughnut",
  () => data.value,
  () => options.value,
);

const display = computed(() => {
  if (props.value === null) return { value: "—", unit: "" };
  if (props.unit && props.unit !== "%") {
    return { value: String(Math.round(props.value)), unit: props.unit };
  }
  return { value: String(Math.round(pct.value)), unit: "%" };
});
</script>

<template>
  <div
    class="flex flex-col items-center rounded-xl border border-surface-200 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900"
  >
    <div class="relative h-28 w-28">
      <canvas ref="canvasEl" />
      <div class="absolute inset-0 flex flex-col items-center justify-center">
        <span
          class="text-xl font-semibold text-surface-900 dark:text-surface-0"
          >{{ display.value }}</span
        >
        <span class="text-xs text-surface-400">{{ display.unit }}</span>
      </div>
    </div>
    <p class="mt-2 text-sm font-medium text-surface-600 dark:text-surface-300">
      {{ label }}
    </p>
  </div>
</template>
