<script setup lang="ts">
import { computed } from "vue";
import Chart from "primevue/chart";
import { useTheme } from "../../../composables/useTheme";
import { seriesColor, fade, axisStyle } from "./chartTheme";

const props = defineProps<{
  labels: string[];
  series: { label: string; data: number[] }[];
}>();

const { isDark } = useTheme();

// Hand Chart.js plain array snapshots, never reactive proxies — it mutates the
// arrays it's given, and a Vue proxy turns that into infinite recursion.
const data = computed(() => ({
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

const options = computed(() => {
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
</script>

<template>
  <div
    class="rounded-xl border border-surface-200 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900"
  >
    <div class="h-56">
      <Chart type="line" :data="data" :options="options" />
    </div>
  </div>
</template>
