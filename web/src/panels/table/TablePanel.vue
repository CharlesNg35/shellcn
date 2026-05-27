<script setup lang="ts">
import { computed, onUnmounted, ref, watch as vueWatch } from "vue";
import DataTable, {
  type DataTableCellEditCompleteEvent,
  type DataTablePageEvent,
  type DataTableSortEvent,
  type DataTableRowClickEvent,
} from "primevue/datatable";
import Column from "primevue/column";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import Menu from "primevue/menu";
import { useToast } from "primevue/usetoast";
import { exportRecords, type ExportFormat } from "../shared/exportData";
import {
  fetchPage,
  runAction,
  watch as watchResource,
} from "../../api/dataSource";
import type {
  Action,
  Column as ColumnSpec,
  DataSource,
  ResourceEvent,
  Row,
  TablePanelConfig,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import { formatBytes } from "../file/fileTypes";
import { inputClass } from "../../primevue/preset";
import { useConfirmAction } from "../../composables/useConfirmAction";
import SkeletonList from "../../components/SkeletonList.vue";
import ActionBar from "../shared/ActionBar.vue";
import PanelError from "../shared/PanelError.vue";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{
  select: [row: Row];
  actionDone: [action: Action, result?: Record<string, unknown>];
}>();

const toast = useToast();
const { confirmDanger } = useConfirmAction();

const INTERNAL = new Set([
  "key",
  "label",
  "leaf",
  "ref",
  "childrenSource",
  "badge",
  "_key",
]);

const rows = ref<Row[]>([]);
const total = ref<number | undefined>();
const loading = ref(false);
const error = ref<string | null>(null);
const filterText = ref("");
const sortField = ref<string | undefined>();
const sortOrder = ref<number | undefined>();
const first = ref(0);
const pageSize = ref(50);
const selectedRow = ref<Row | null>(null);
const actionOutput = ref<{
  title: string;
  output: string;
  truncated: boolean;
} | null>(null);

const declaredColumns = computed(
  () => (props.config as TablePanelConfig | undefined)?.columns,
);
const tableConfig = computed(
  () => props.config as TablePanelConfig | undefined,
);
const actionIds = computed(() => tableConfig.value?.actionIds ?? []);
const rowActionIds = computed(() => tableConfig.value?.rowActionIds ?? []);
const globalActions = computed(() => resolveActions(actionIds.value));
const rowActions = computed(() => resolveActions(rowActionIds.value));
const emptyText = computed(() => tableConfig.value?.emptyText ?? "No rows.");

// Export the loaded rows (CSV/JSON). Opt-in per plugin via the manifest, so a
// table never exposes export unless the plugin declares it.
const canExport = computed(() => Boolean(tableConfig.value?.exportable));
const exportMenu = ref<{ toggle: (event: Event) => void } | null>(null);
function runExport(format: ExportFormat): void {
  const keys = columns.value.map((c) => c.key);
  exportRecords(
    props.source?.routeId ?? "export",
    keys,
    rows.value as Record<string, unknown>[],
    format,
  );
}
const exportItems = [
  { label: "Export CSV", icon: "pi", command: () => runExport("csv") },
  { label: "Export JSON", icon: "pi", command: () => runExport("json") },
];

// --- editable data grid -------------------------------------------------
// Editing is driven entirely by the manifest: a table is editable when it
// declares `editable` plus the mutation routes. Per-row update/delete need a
// row key (a `_key` map on the row, or the configured `rowKey` columns); rows
// without one stay read-only so we never mutate the wrong record.
const insertSource = computed(() => tableConfig.value?.insert);
const updateSource = computed(() => tableConfig.value?.update);
const deleteSource = computed(() => tableConfig.value?.delete);
const editable = computed(
  () =>
    Boolean(tableConfig.value?.editable) &&
    Boolean(insertSource.value || updateSource.value || deleteSource.value),
);
const editableCells = computed(() => editable.value && !!updateSource.value);

function keyFor(row: Row): Record<string, unknown> | null {
  const explicit = row._key;
  if (explicit && typeof explicit === "object") {
    return explicit as Record<string, unknown>;
  }
  const cols = tableConfig.value?.rowKey;
  if (cols?.length) {
    const key: Record<string, unknown> = {};
    for (const c of cols) key[c] = row[c];
    return key;
  }
  return null;
}

function columnReadOnly(col: ColumnSpec): boolean {
  return col.readOnly === true;
}

function coerce(prev: unknown, next: unknown): unknown {
  if (typeof prev === "number") {
    if (next === "" || next === null) return null;
    const n = Number(next);
    return Number.isNaN(n) ? next : n;
  }
  if (typeof prev === "boolean") return next === true || next === "true";
  if (next === "") return null;
  return next;
}

async function mutate(
  src: DataSource,
  body: Record<string, unknown>,
): Promise<void> {
  await runAction(
    props.connectionId,
    src.routeId,
    { resource: props.resource },
    body,
    src.params ?? {},
    src.method ?? "POST",
  );
}

async function onCellEditComplete(
  e: DataTableCellEditCompleteEvent,
): Promise<void> {
  const src = updateSource.value;
  if (!src) return;
  const data = e.data as Row;
  const field = e.field;
  const key = keyFor(data);
  if (!key) {
    data[field] = e.value;
    toast.add({
      severity: "warn",
      summary: "Read-only row",
      detail: "This table has no primary key, so rows cannot be edited.",
      life: 5000,
    });
    return;
  }
  const value = coerce(e.value, e.newValue);
  if (value === e.value) return;
  data[field] = value;
  try {
    await mutate(src, { key, values: { [field]: value } });
  } catch (err) {
    data[field] = e.value;
    toast.add({
      severity: "error",
      summary: "Update failed",
      detail: (err as Error).message,
      life: 6000,
    });
    return;
  }
  await load(first.value);
}

function askDeleteRow(row: Row): void {
  const src = deleteSource.value;
  const key = keyFor(row);
  if (!src || !key) return;
  confirmDanger({
    header: "Delete row",
    message: "Delete this row? This cannot be undone.",
    accept: async () => {
      try {
        await mutate(src, { key });
        toast.add({ severity: "success", summary: "Row deleted", life: 3000 });
        await load(first.value);
      } catch (err) {
        toast.add({
          severity: "error",
          summary: "Delete failed",
          detail: (err as Error).message,
          life: 6000,
        });
      }
    },
  });
}

const showInsert = ref(false);
const insertDraft = ref<Record<string, string>>({});
const inserting = ref(false);

function openInsert(): void {
  insertDraft.value = {};
  for (const col of columns.value) {
    if (!INTERNAL.has(col.key)) insertDraft.value[col.key] = "";
  }
  showInsert.value = true;
}

async function submitInsert(): Promise<void> {
  const src = insertSource.value;
  if (!src) return;
  const values: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(insertDraft.value)) {
    if (v !== "") values[k] = v;
  }
  inserting.value = true;
  try {
    await mutate(src, { values });
    showInsert.value = false;
    toast.add({ severity: "success", summary: "Row added", life: 3000 });
    await load(0);
  } catch (err) {
    toast.add({
      severity: "error",
      summary: "Insert failed",
      detail: (err as Error).message,
      life: 6000,
    });
  } finally {
    inserting.value = false;
  }
}

// -----------------------------------------------------------------------

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
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

async function load(targetFirst = first.value): Promise<void> {
  if (!props.source) return;
  loading.value = true;
  error.value = null;
  selectedRow.value = null;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      props.source,
      { resource: props.resource },
      {
        cursor: targetFirst > 0 ? String(targetFirst) : "",
        limit: pageSize.value,
        filter: filterText.value ? { q: filterText.value } : undefined,
        sort: sortField.value
          ? [{ field: sortField.value, desc: sortOrder.value === -1 }]
          : undefined,
      },
    );
    rows.value = page.items;
    total.value = page.total;
    first.value = targetFirst;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function onSort(e: DataTableSortEvent): void {
  sortField.value = (e.sortField as string) ?? undefined;
  sortOrder.value = e.sortOrder ?? undefined;
  void load(0);
}

function onPage(e: DataTablePageEvent): void {
  first.value = e.first;
  pageSize.value = e.rows;
  void load(e.first);
}

function onRowClick(e: DataTableRowClickEvent): void {
  const row = e.data as Row;
  selectedRow.value = row;
  if (row.ref) emit("select", row);
}

function rowClass(row: Row): string {
  return row.ref ? "cursor-pointer" : "";
}

function resolveActions(ids: string[]): Action[] {
  return ids
    .map((id) => props.actions?.find((a) => a.id === id))
    .filter((a): a is Action => Boolean(a));
}

async function onActionDone(
  action: Action,
  result?: Record<string, unknown>,
): Promise<void> {
  if (typeof result?.output === "string" && !action.onSuccess?.selectTab) {
    actionOutput.value = {
      title: action.label,
      output: result.output,
      truncated: result.truncated === true,
    };
  }
  await load(first.value);
  emit("actionDone", action, result);
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
  const ds = tableConfig.value?.watch as DataSource | undefined;
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
    first.value = 0;
    load(0);
    startWatch();
  },
  { immediate: true },
);

let debounce: ReturnType<typeof setTimeout> | undefined;
function onFilter(): void {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(() => load(0), 250);
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
      <div class="w-56">
        <input
          v-model="filterText"
          type="search"
          placeholder="Filter…"
          aria-label="Filter rows"
          :class="inputClass"
          @input="onFilter"
        />
      </div>
      <span v-if="total != null" class="text-xs text-surface-400"
        >{{ total }} total</span
      >
      <Button
        v-if="editable && insertSource"
        type="button"
        severity="secondary"
        :disabled="loading || !columns.length"
        :title="columns.length ? 'Add a row' : 'Load or define columns first'"
        @click="openInsert"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
        Add row
      </Button>
      <ActionBar
        v-if="globalActions.length"
        :connection-id="connectionId"
        :actions="globalActions"
        :resource="resource"
        @done="onActionDone"
      />
      <ActionBar
        v-if="rowActions.length && selectedRow?.ref"
        :connection-id="connectionId"
        :actions="rowActions"
        :resource="selectedRow.ref"
        @done="onActionDone"
      />
      <div class="ml-auto flex items-center gap-2">
        <Button
          v-if="canExport"
          type="button"
          severity="secondary"
          :disabled="!rows.length"
          title="Export loaded rows"
          aria-haspopup="true"
          @click="exportMenu?.toggle($event)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="14" />
          Export
        </Button>
        <Menu v-if="canExport" ref="exportMenu" :model="exportItems" popup />
        <Button
          type="button"
          :disabled="loading"
          severity="secondary"
          @click="load(first)"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'refresh-cw' }"
            :size="14"
            :loading="loading"
          />
          Refresh
        </Button>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-hidden">
      <PanelError
        v-if="error"
        :message="error"
        retryable
        @retry="load(first)"
      />
      <SkeletonList v-else-if="loading && !rows.length" :rows="8" />
      <DataTable
        v-else
        :value="rows"
        :data-key="editable ? undefined : 'ref.uid'"
        :edit-mode="editableCells ? 'cell' : undefined"
        lazy
        paginator
        :first="first"
        :rows="pageSize"
        :total-records="total ?? rows.length"
        :rows-per-page-options="[25, 50, 100, 250]"
        paginator-template="RowsPerPageDropdown FirstPageLink PrevPageLink CurrentPageReport NextPageLink LastPageLink"
        current-page-report-template="{first} to {last} of {totalRecords}"
        removable-sort
        :sort-field="sortField"
        :sort-order="sortOrder"
        scrollable
        scroll-height="flex"
        :row-class="rowClass"
        @sort="onSort"
        @page="onPage"
        @row-click="onRowClick"
        @cell-edit-complete="onCellEditComplete"
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
          <template
            v-if="editableCells && !columnReadOnly(col)"
            #editor="{ data, field }"
          >
            <Select
              v-if="typeof data[field] === 'boolean'"
              v-model="data[field]"
              :options="[
                { label: 'true', value: true },
                { label: 'false', value: false },
              ]"
              option-label="label"
              option-value="value"
              class="w-full"
            />
            <InputText
              v-else
              v-model="data[field]"
              :class="inputClass"
              autofocus
            />
          </template>
        </Column>
        <Column
          v-if="editable && deleteSource"
          :pt="{ bodyCell: 'w-12 text-right' }"
        >
          <template #body="{ data }">
            <Button
              v-if="keyFor(data as Row)"
              type="button"
              text
              rounded
              severity="danger"
              title="Delete row"
              aria-label="Delete row"
              @click.stop="askDeleteRow(data as Row)"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'trash-2' }"
                :size="15"
              />
            </Button>
          </template>
        </Column>
        <template #empty>{{ emptyText }}</template>
      </DataTable>
    </div>

    <Dialog
      v-model:visible="showInsert"
      modal
      header="Add row"
      :dismissable-mask="true"
      :pt="{
        root: 'w-full max-w-lg overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-2xl dark:border-surface-800 dark:bg-surface-900',
      }"
    >
      <div class="flex max-h-[60vh] flex-col gap-3 overflow-auto p-1">
        <label
          v-for="col in columns.filter((c) => !INTERNAL.has(c.key))"
          :key="col.key"
          class="flex flex-col gap-1 text-sm"
        >
          <span class="text-surface-500">{{ col.label }}</span>
          <InputText
            v-model="insertDraft[col.key]"
            :class="inputClass"
            placeholder="NULL"
          />
        </label>
      </div>
      <template #footer>
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          @click="showInsert = false"
        />
        <Button
          type="button"
          label="Add row"
          :loading="inserting"
          :disabled="inserting"
          @click="submitInsert"
        />
      </template>
    </Dialog>

    <Dialog
      :visible="!!actionOutput"
      modal
      :header="actionOutput?.title"
      :dismissable-mask="true"
      :pt="{
        root: 'w-full max-w-3xl overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-2xl dark:border-surface-800 dark:bg-surface-900',
      }"
      @update:visible="(v) => !v && (actionOutput = null)"
    >
      <pre
        class="max-h-[60vh] overflow-auto rounded-lg bg-surface-950 p-4 text-xs leading-relaxed text-surface-100"
        >{{ actionOutput?.output || "(no output)" }}</pre
      >
      <p v-if="actionOutput?.truncated" class="mt-2 text-xs text-amber-500">
        Output truncated.
      </p>
      <template #footer>
        <Button
          type="button"
          label="Close"
          severity="secondary"
          @click="actionOutput = null"
        />
      </template>
    </Dialog>
  </div>
</template>
