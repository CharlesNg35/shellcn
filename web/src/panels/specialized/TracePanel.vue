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
import { useRefreshableSource } from "../shared/useRefreshableSource";

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

// A distributed trace has no natural bound, so the panel renders a prefix of the
// waterfall and says how much it dropped.
const MAX_SPANS = 2000;
const VIRTUAL_THRESHOLD = 100;
// Two stacked lines (text-sm 20px + text-xs 16px) inside the preset `bodyCell`
// (py-2 = 16px) plus its 1px bottom border.
const ROW_HEIGHT = 53;

const filterText = ref("");
const selected = ref<SpanRow | null>(null);
const traceConfig = computed(
  () => props.config as TracePanelConfig | undefined,
);

async function loadTrace(): Promise<TracePayload> {
  if (!props.source) return {};
  return fetchDoc<TracePayload>(props.connectionId, props.source, {
    resource: props.resource,
    record: props.record,
  });
}

const {
  data: payload,
  refreshing,
  error,
  showInitialLoader,
  blockingError,
  load,
  reset,
} = useRefreshableSource<TracePayload>(loadTrace, {
  initialValue: () => ({}),
  connectionId: () => props.connectionId,
});

function spanStart(span: TraceSpan): number {
  if (typeof span.startMs === "number") return span.startMs;
  if (typeof span.startTime === "number") return span.startTime;
  if (typeof span.startTime === "string") {
    const parsed = Date.parse(span.startTime);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

const allRows = computed<SpanRow[]>(() => {
  const spans = payload.value.spans ?? [];
  const byParent = new Map<string, TraceSpan[]>();
  for (const span of spans) {
    const key = span.parentId ?? "";
    const group = byParent.get(key);
    if (group) group.push(span);
    else byParent.set(key, [span]);
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

const filteredRows = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return allRows.value;
  return allRows.value.filter((span) =>
    [span.name, span.service, span.status].some((value) =>
      String(value ?? "")
        .toLowerCase()
        .includes(q),
    ),
  );
});

const truncated = computed(() => filteredRows.value.length > MAX_SPANS);
const visibleRows = computed(() =>
  truncated.value ? filteredRows.value.slice(0, MAX_SPANS) : filteredRows.value,
);
const rowVirtualScroller = computed(() =>
  visibleRows.value.length > VIRTUAL_THRESHOLD
    ? { itemSize: ROW_HEIGHT }
    : undefined,
);

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

async function refreshTrace(): Promise<void> {
  const next = await load();
  if (next) selected.value = null;
}

function selectRow(event: { data: unknown }): void {
  const row = event.data as SpanRow;
  selected.value = selected.value?.id === row.id ? null : row;
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
    selected.value = null;
    reset();
    void refreshTrace();
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
      <span class="text-xs text-surface-400">{{ allRows.length }} spans</span>
      <span
        v-if="truncated"
        data-test="trace-truncated"
        class="text-xs text-amber-600 dark:text-amber-400"
      >
        Showing the first {{ MAX_SPANS }} of {{ filteredRows.length }} — filter
        to narrow the trace.
      </span>
      <Button
        type="button"
        severity="secondary"
        class="ml-auto"
        :disabled="refreshing"
        @click="refreshTrace"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="refreshing"
        />
        Refresh
      </Button>
    </div>

    <div
      class="grid min-h-0 flex-1"
      :class="
        selected ? 'grid-cols-[minmax(0,1fr)_minmax(0,20rem)]' : 'grid-cols-1'
      "
    >
      <div class="min-h-0 overflow-hidden">
        <SkeletonList v-if="showInitialLoader" />
        <PanelError
          v-else-if="blockingError"
          :message="blockingError"
          retryable
          @retry="refreshTrace"
        />
        <DataTable
          v-else
          :value="visibleRows"
          data-key="id"
          scrollable
          scroll-height="flex"
          selection-mode="single"
          :virtual-scroller-options="rowVirtualScroller"
          @row-click="selectRow"
        >
          <template v-if="error" #header>
            <PanelError :message="error" retryable @retry="refreshTrace" />
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
        v-if="selected"
        class="min-h-0 overflow-auto border-l border-surface-200 p-4 dark:border-surface-800"
      >
        <div class="flex items-start justify-between gap-2">
          <p class="text-xs text-surface-400 uppercase">
            {{ spanService(selected) }}
          </p>
          <Button
            type="button"
            severity="secondary"
            variant="text"
            size="small"
            aria-label="Close span details"
            @click="selected = null"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
          </Button>
        </div>
        <h3 class="mt-1 font-semibold text-surface-900 dark:text-surface-0">
          {{ selected.name }}
        </h3>
        <dl class="mt-4 space-y-2 text-sm">
          <div>
            <dt class="text-surface-400">Span ID</dt>
            <dd class="font-mono text-xs wrap-break-word">{{ selected.id }}</dd>
          </div>
          <div v-if="selected.parentId">
            <dt class="text-surface-400">Parent</dt>
            <dd class="font-mono text-xs wrap-break-word">
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
          <Column field="key" header="Tag" style="width: 40%" />
          <Column header="Value">
            <template #body="{ data }">
              <span class="wrap-break-word">{{ String(data.value) }}</span>
            </template>
          </Column>
        </DataTable>
      </aside>
    </div>
  </div>
</template>
