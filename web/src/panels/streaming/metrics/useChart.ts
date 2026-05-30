import { onBeforeUnmount, onMounted, watch, type Ref } from "vue";
import Chart from "chart.js/auto";
import type { ChartData, ChartOptions, ChartType } from "chart.js";

// Owns one persistent Chart.js instance and mutates it in place on data/option
// changes, so a streaming chart animates the diff instead of being rebuilt every
// frame. (PrimeVue's <Chart> destroys and recreates on every data change, which
// is why a live series flickers with no transition.)
export function useChart(
  canvas: Ref<HTMLCanvasElement | null>,
  type: ChartType,
  data: () => ChartData,
  options: () => ChartOptions,
) {
  let chart: Chart | null = null;

  function context(el: HTMLCanvasElement): CanvasRenderingContext2D | null {
    try {
      return el.getContext("2d");
    } catch {
      return null; // jsdom/test env without a canvas backend
    }
  }

  onMounted(() => {
    const el = canvas.value;
    if (!el || !context(el)) return;
    chart = new Chart(el, { type, data: data(), options: options() });
  });

  watch(data, (next) => {
    if (!chart) return;
    chart.data.labels = next.labels ?? [];
    next.datasets.forEach((ds, i) => {
      if (chart!.data.datasets[i]) Object.assign(chart!.data.datasets[i], ds);
      else chart!.data.datasets.push(ds);
    });
    chart.data.datasets.length = next.datasets.length;
    chart.update();
  });

  watch(options, (next) => {
    if (!chart) return;
    chart.options = next;
    chart.update();
  });

  onBeforeUnmount(() => {
    chart?.destroy();
    chart = null;
  });
}
