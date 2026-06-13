<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { useStream } from "@/composables/useStream";
import type { PanelProps } from "../core/types";
import type {
  MetricGauge,
  MetricSeries,
  MetricStat,
  MetricUsage,
  MetricsPanelConfig,
  Row,
} from "@/types/projection";
import StreamStatusBar from "./StreamStatusBar.vue";
import StatCard from "./metrics/StatCard.vue";
import GaugeChart from "./metrics/GaugeChart.vue";
import SeriesChart from "./metrics/SeriesChart.vue";
import UsageRows from "./metrics/UsageRows.vue";

const props = defineProps<PanelProps>();
const cfg = computed(
  () => (props.config as MetricsPanelConfig | undefined) ?? {},
);

const stats = computed<MetricStat[]>(() => cfg.value.stats ?? []);
const gauges = computed<MetricGauge[]>(() => cfg.value.gauges ?? []);
const usage = computed<MetricUsage[]>(() => cfg.value.usage ?? []);
const usageMetricKeys = computed(() => {
  const keys = new Set<string>();
  for (const field of usage.value) {
    keys.add(field.key);
    if (field.usage?.percentKey) keys.add(field.usage.percentKey);
    if (field.usage?.usedKey) keys.add(field.usage.usedKey);
  }
  return keys;
});
const visibleGauges = computed<MetricGauge[]>(() =>
  gauges.value.filter((gauge) => !usageMetricKeys.value.has(gauge.key)),
);
const series = computed<MetricSeries[]>(() => cfg.value.series ?? []);
const hasMetrics = computed(
  () =>
    stats.value.length +
      visibleGauges.value.length +
      usage.value.length +
      series.value.length >
    0,
);
const historyLimit = computed(() =>
  cfg.value.history && cfg.value.history > 0 ? cfg.value.history : 60,
);

const latest = reactive<Record<string, number>>({});
const histories = reactive<Record<string, number[]>>({});
const labels = ref<string[]>([]);
const reconnecting = ref(false);

function onFrame(raw: string): void {
  let frame: Record<string, unknown>;
  try {
    frame = JSON.parse(raw);
  } catch {
    return;
  }
  let changed = false;
  for (const [k, v] of Object.entries(frame)) {
    if (typeof v === "number") {
      latest[k] = v;
      changed = true;
    }
  }
  if (!changed) return;
  labels.value.push(new Date().toLocaleTimeString());
  if (labels.value.length > historyLimit.value) labels.value.shift();
  for (const s of series.value) {
    const arr = histories[s.key] ?? (histories[s.key] = []);
    arr.push(latest[s.key] ?? 0);
    if (arr.length > historyLimit.value) arr.shift();
  }
}

const seriesData = computed(() =>
  series.value.map((s) => ({
    label: s.unit ? `${s.label ?? s.key} (${s.unit})` : (s.label ?? s.key),
    data: histories[s.key] ?? [],
  })),
);
const latestRow = computed<Row>(() => ({ ...latest }));

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
    <div class="min-h-0 flex-1 space-y-4 overflow-auto p-4">
      <div v-if="stats.length" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          v-for="s in stats"
          :key="s.key"
          :label="s.label ?? s.key"
          :value="latest[s.key] ?? null"
          :unit="s.unit"
        />
      </div>
      <div
        v-if="visibleGauges.length"
        class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
      >
        <GaugeChart
          v-for="(g, i) in visibleGauges"
          :key="g.key"
          :label="g.label ?? g.key"
          :value="latest[g.key] ?? null"
          :max="g.max ?? 100"
          :unit="g.unit"
          :color-index="i"
        />
      </div>
      <UsageRows v-if="usage.length" :fields="usage" :values="latestRow" />
      <SeriesChart v-if="series.length" :labels="labels" :series="seriesData" />
      <div
        v-if="!hasMetrics"
        class="flex h-full items-center justify-center text-sm text-surface-400"
      >
        No metrics configured.
      </div>
    </div>
  </div>
</template>
