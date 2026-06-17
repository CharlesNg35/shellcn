<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import InputText from "primevue/inputtext";
import { fetchDoc } from "@/api/dataSource";
import type { TracePanelConfig } from "@/types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import AppIcon from "@/components/AppIcon.vue";

interface TraceSpan {
  id: string;
  parentId?: string;
  name: string;
  service?: string;
  startTime?: string | number;
  startMs?: number;
  durationMs: number;
  status?: string;
  tags?: Record<string, unknown>;
}

interface TracePayload {
  traceId?: string;
  spans?: TraceSpan[];
}

type SpanRow = TraceSpan & {
  depth: number;
  offset: number;
  width: number;
};

const props = defineProps<PanelProps>();

const loadedOnce = ref(false);
const refreshing = ref(false);
const error = ref<string | null>(null);
const payload = ref<TracePayload>({});
const filterText = ref("");
const selected = ref<SpanRow | null>(null);
const traceConfig = computed(
  () => props.config as TracePanelConfig | undefined,
);
const showInitialLoader = computed(() => refreshing.value && !loadedOnce.value);
const blockingError = computed(() => error.value && !loadedOnce.value);

function spanStart(span: TraceSpan): number {
  if (typeof span.startMs === "number") return span.startMs;
  if (typeof span.startTime === "number") return span.startTime;
  if (typeof span.startTime === "string") {
    const parsed = Date.parse(span.startTime);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

const rows = computed<SpanRow[]>(() => {
  const spans = payload.value.spans ?? [];
  const byParent = new Map<string, TraceSpan[]>();
  for (const span of spans) {
    const key = span.parentId ?? "";
    byParent.set(key, [...(byParent.get(key) ?? []), span]);
  }
  for (const group of byParent.values()) {
    group.sort((a, b) => spanStart(a) - spanStart(b));
  }
  const start = spans.length ? Math.min(...spans.map(spanStart)) : 0;
  const end = spans.length
    ? Math.max(...spans.map((span) => spanStart(span) + span.durationMs))
    : 1;
  const total = Math.max(end - start, 1);
  const out: SpanRow[] = [];
  const visit = (span: TraceSpan, depth: number): void => {
    out.push({
      ...span,
      depth,
      offset: ((spanStart(span) - start) / total) * 100,
      width: Math.max((span.durationMs / total) * 100, 0.5),
    });
    for (const child of byParent.get(span.id) ?? []) visit(child, depth + 1);
  };
  const spanIds = new Set(spans.map((span) => span.id));
  const roots = spans.filter(
    (span) => !span.parentId || !spanIds.has(span.parentId),
  );
  for (const root of roots) visit(root, 0);
  return out;
});

const visibleRows = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return rows.value;
  return rows.value.filter((span) =>
    [span.name, span.service, span.status].some((value) =>
      String(value ?? "")
        .toLowerCase()
        .includes(q),
    ),
  );
});

const tags = computed(() =>
  selected.value?.tags
    ? Object.entries(selected.value.tags).map(([key, value]) => ({
        key,
        value,
      }))
    : [],
);

function spanService(row: SpanRow): string {
  const field = traceConfig.value?.serviceField;
  const value = field
    ? (row as unknown as Record<string, unknown>)[field]
    : row.service;
  return String(value ?? row.service ?? "unknown");
}

function selectRow(event: { data: unknown }): void {
  selected.value = event.data as SpanRow;
}

async function load(): Promise<void> {
  if (!props.source) {
    loadedOnce.value = true;
    return;
  }
  if (refreshing.value) return;
  refreshing.value = true;
  error.value = null;
  try {
    payload.value = await fetchDoc<TracePayload>(
      props.connectionId,
      props.source,
      {
        resource: props.resource,
        record: props.record,
      },
    );
    selected.value = null;
    loadedOnce.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    refreshing.value = false;
  }
}

watch(
  () => [
    props.connectionId,
    props.resource?.uid,
    props.source?.routeId,
    JSON.stringify(props.source?.params ?? {}),
    JSON.stringify(props.record ?? {}),
  ],
  () => {
    payload.value = {};
    selected.value = null;
    loadedOnce.value = false;
    void load();
  },
  {
    immediate: true,
  },
);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex flex-wrap items-center gap-3 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <InputText
        v-model="filterText"
        placeholder="Filter spans"
        aria-label="Filter spans"
        class="w-56"
      />
      <span class="text-xs text-surface-400">{{ rows.length }} spans</span>
      <Button
        type="button"
        severity="secondary"
        class="ml-auto"
        :disabled="refreshing"
        @click="load"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="refreshing"
        />
        Refresh
      </Button>
    </div>

    <div class="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_20rem]">
      <div class="min-h-0 overflow-hidden">
        <SkeletonList v-if="showInitialLoader" />
        <PanelError
          v-else-if="blockingError"
          :message="error ?? ''"
          retryable
          @retry="load"
        />
        <DataTable
          v-else
          :value="visibleRows"
          data-key="id"
          scrollable
          scroll-height="flex"
          selection-mode="single"
          @row-click="selectRow"
        >
          <template v-if="error" #header>
            <PanelError :message="error" retryable @retry="load" />
          </template>
          <template #empty>No spans.</template>
          <Column header="Span">
            <template #body="{ data }">
              <div
                class="truncate text-sm"
                :style="{ paddingLeft: `${(data as SpanRow).depth * 14}px` }"
              >
                {{ (data as SpanRow).name }}
              </div>
              <div class="truncate text-xs text-surface-400">
                {{ spanService(data as SpanRow) }}
              </div>
            </template>
          </Column>
          <Column header="Duration" style="width: 7rem">
            <template #body="{ data }">
              {{ (data as SpanRow).durationMs.toFixed(1) }} ms
            </template>
          </Column>
          <Column header="Timeline" style="width: 45%">
            <template #body="{ data }">
              <div
                class="relative h-6 rounded bg-surface-100 dark:bg-surface-800"
              >
                <div
                  class="absolute top-1 h-4 rounded bg-primary-500"
                  :class="
                    (data as SpanRow).status === 'error' ? 'bg-rose-400' : ''
                  "
                  :style="{
                    left: `${(data as SpanRow).offset}%`,
                    width: `${(data as SpanRow).width}%`,
                  }"
                />
              </div>
            </template>
          </Column>
        </DataTable>
      </div>

      <aside
        class="min-h-0 overflow-auto border-l border-surface-200 p-4 dark:border-surface-800"
      >
        <p v-if="!selected" class="text-sm text-surface-400">Select a span.</p>
        <template v-else>
          <p class="text-xs text-surface-400 uppercase">
            {{ spanService(selected) }}
          </p>
          <h3 class="mt-1 font-semibold text-surface-900 dark:text-surface-0">
            {{ selected.name }}
          </h3>
          <dl class="mt-4 space-y-2 text-sm">
            <div>
              <dt class="text-surface-400">Span ID</dt>
              <dd class="font-mono text-xs break-all">{{ selected.id }}</dd>
            </div>
            <div v-if="selected.parentId">
              <dt class="text-surface-400">Parent</dt>
              <dd class="font-mono text-xs break-all">
                {{ selected.parentId }}
              </dd>
            </div>
            <div>
              <dt class="text-surface-400">Duration</dt>
              <dd>{{ selected.durationMs.toFixed(1) }} ms</dd>
            </div>
          </dl>
          <DataTable
            v-if="tags.length"
            :value="tags"
            class="mt-4"
            scrollable
            scroll-height="16rem"
          >
            <Column field="key" header="Tag" />
            <Column header="Value">
              <template #body="{ data }">
                <span class="break-all">{{ String(data.value) }}</span>
              </template>
            </Column>
          </DataTable>
        </template>
      </aside>
    </div>
  </div>
</template>
