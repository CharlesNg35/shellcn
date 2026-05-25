<script setup lang="ts">
import { computed, onUnmounted, ref, watch as vueWatch } from "vue";
import DataTable, {
  type DataTableSortEvent,
  type DataTableRowClickEvent,
} from "primevue/datatable";
import Column from "primevue/column";
import { fetchPage, watch as watchResource } from "../api/dataSource";
import type {
  Column as ColumnSpec,
  DataSource,
  ResourceEvent,
  Row,
} from "../types/projection";
import type { PanelProps } from "./types";
import { formatBytes } from "./file/fileTypes";
import SkeletonList from "../components/SkeletonList.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{ select: [row: Row] }>();

const INTERNAL = new Set([
  "key",
  "label",
  "leaf",
  "ref",
  "childrenSource",
  "badge",
]);

const rows = ref<Row[]>([]);
const nextCursor = ref("");
const total = ref<number | undefined>();
const loading = ref(false);
const error = ref<string | null>(null);
const filterText = ref("");
const sortField = ref<string | undefined>();
const sortOrder = ref<number | undefined>();

const declaredColumns = computed(
  () => props.config?.columns as ColumnSpec[] | undefined,
);

const columns = computed<ColumnSpec[]>(() => {
  if (declaredColumns.value?.length) return declaredColumns.value;
  const sample = rows.value[0];
  if (!sample) return [];
  return Object.keys(sample)
    .filter((k) => !INTERNAL.has(k))
    .map((key) => ({ key, label: key }));
});

function display(row: Row, col: ColumnSpec): string {
  const v = row[col.key];
  if (v === undefined || v === null || v === "") return "—";
  if (col.type === "bytes" && typeof v === "number") return formatBytes(v);
  if (col.type === "datetime" && typeof v === "string")
    return new Date(v).toLocaleString();
  return String(v);
}

async function load(reset: boolean): Promise<void> {
  if (!props.source) return;
  loading.value = true;
  error.value = null;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      props.source,
      { resource: props.resource },
      {
        cursor: reset ? "" : nextCursor.value,
        limit: 50,
        filter: filterText.value ? { q: filterText.value } : undefined,
        sort: sortField.value
          ? [{ field: sortField.value, desc: sortOrder.value === -1 }]
          : undefined,
      },
    );
    rows.value = reset ? page.items : [...rows.value, ...page.items];
    nextCursor.value = page.nextCursor;
    total.value = page.total;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function onSort(e: DataTableSortEvent): void {
  sortField.value = (e.sortField as string) ?? undefined;
  sortOrder.value = e.sortOrder ?? undefined;
  load(true);
}

function onRowClick(e: DataTableRowClickEvent): void {
  const row = e.data as Row;
  if (row.ref) emit("select", row);
}

function applyEvent(ev: ResourceEvent): void {
  const idx = rows.value.findIndex((r) => r.ref?.uid === ev.ref.uid);
  if (ev.type === "deleted") {
    if (idx >= 0) rows.value.splice(idx, 1);
  } else if (ev.type === "added" && idx < 0 && ev.resource) {
    rows.value.unshift({ ...(ev.resource as Row), ref: ev.ref });
  } else if (idx >= 0 && ev.resource) {
    rows.value[idx] = { ...rows.value[idx], ...(ev.resource as Row) };
  }
}

let stopWatch: (() => void) | undefined;
function startWatch(): void {
  const ds = props.config?.watch as DataSource | undefined;
  stopWatch?.();
  stopWatch = ds
    ? watchResource(
        props.connectionId,
        ds,
        { resource: props.resource },
        applyEvent,
      )
    : undefined;
}

vueWatch(
  () => [props.connectionId, props.source?.routeId, props.resource?.uid],
  () => {
    filterText.value = "";
    sortField.value = undefined;
    load(true);
    startWatch();
  },
  { immediate: true },
);

let debounce: ReturnType<typeof setTimeout> | undefined;
function onFilter(): void {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(() => load(true), 250);
}

onUnmounted(() => {
  stopWatch?.();
  if (debounce) clearTimeout(debounce);
});
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center gap-3 border-b border-surface-200 px-4 py-2 dark:border-surface-800"
    >
      <input
        v-model="filterText"
        type="search"
        placeholder="Filter…"
        class="w-56 rounded-md border border-surface-300 bg-surface-0 px-2.5 py-1 text-sm outline-none focus:border-primary-400 dark:border-surface-700 dark:bg-surface-950"
        @input="onFilter"
      />
      <span v-if="total != null" class="text-xs text-surface-400"
        >{{ total }} total</span
      >
      <button
        type="button"
        :disabled="loading"
        class="ml-auto rounded-md border border-surface-300 px-3 py-1 text-sm hover:bg-surface-100 disabled:opacity-50 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="load(true)"
      >
        Refresh
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-hidden">
      <p v-if="error" class="p-4 text-sm text-red-500">{{ error }}</p>
      <SkeletonList v-else-if="loading && !rows.length" :rows="8" />
      <DataTable
        v-else
        :value="rows"
        data-key="ref.uid"
        lazy
        removable-sort
        :sort-field="sortField"
        :sort-order="sortOrder"
        scrollable
        scroll-height="flex"
        @sort="onSort"
        @row-click="onRowClick"
      >
        <Column
          v-for="col in columns"
          :key="col.key"
          :field="col.key"
          :header="col.label"
          :sortable="col.sortable"
        >
          <template #body="{ data }">
            <span
              v-if="col.type === 'badge'"
              class="rounded-full bg-surface-100 px-2 py-0.5 text-xs dark:bg-surface-800"
              >{{ display(data as Row, col) }}</span
            >
            <template v-else>{{ display(data as Row, col) }}</template>
          </template>
        </Column>
        <template #empty>No rows.</template>
      </DataTable>
    </div>

    <div
      v-if="nextCursor"
      class="border-t border-surface-200 p-2 text-center dark:border-surface-800"
    >
      <button
        type="button"
        :disabled="loading"
        class="rounded-md border border-surface-300 px-3 py-1 text-sm hover:bg-surface-100 disabled:opacity-50 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="load(false)"
      >
        {{ loading ? "Loading…" : "Load more" }}
      </button>
    </div>
  </div>
</template>
