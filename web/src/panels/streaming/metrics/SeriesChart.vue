<script setup lang="ts">
import { computed, ref } from "vue";
import type { ChartData, ChartOptions } from "chart.js";
import { useTheme } from "../../../composables/useTheme";
import { seriesColor, fade, axisStyle } from "./chartTheme";
import { useChart } from "./useChart";

const props = defineProps<{
  labels: string[];
  series: { label: string; data: number[] }[];
}>();

const { isDark } = useTheme();

const data = computed<ChartData>(() => ({
  labels: [...props.labels],
  datasets: props.series.map((s, i) => ({
    label: s.label,
    data: [...s.data],
    borderColor: seriesColor(i),
    backgroundColor: fade(seriesColor(i)),
    fill: true,
    tension: 0.35,
    borderWidth: 2,
    pointRadius: 0,
    pointHoverRadius: 3,
  })),
}));

const options = computed<ChartOptions>(() => {
  const ax = axisStyle(isDark.value);
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: "index", intersect: false },
    plugins: {
      legend: {
        display: props.series.length > 1,
        position: "top",
        labels: { color: ax.tick, usePointStyle: true, boxWidth: 8 },
      },
    },
    scales: {
      x: {
        ticks: { color: ax.tick, maxTicksLimit: 6 },
        grid: { display: false },
      },
      y: {
        beginAtZero: true,
        ticks: { color: ax.tick },
        grid: { color: ax.grid },
      },
    },
    animation: { duration: 300 },
  };
});

const canvasEl = ref<HTMLCanvasElement | null>(null);
useChart(
  canvasEl,
  "line",
  () => data.value,
  () => options.value,
);
</script>

<template>
  <div
    class="rounded-xl border border-surface-200 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900"
  >
    <div class="h-56">
      <canvas ref="canvasEl" />
    </div>
  </div>
</template>
