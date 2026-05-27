<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch as vueWatch } from "vue";
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
  ResourceRef,
  Row,
  TablePanelConfig,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import { formatBytes } from "../file/fileTypes";
import { dialogRoot, inputClass } from "../../primevue/preset";
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

// Framework-reserved row keys the grid never renders as data columns. Plugins
// hide their own fields declaratively via config.hiddenColumns instead.
const RESERVED = new Set([
  "key",
  "label",
  "leaf",
  "ref",
  "childrenSource",
  "badge",
  "_key",
  "_links",
  "__rid",
]);

// Stable per-row id used to key staged edits/inserts/deletes by row identity
// without relying on object references (which change across reactive proxies).
const RID = "__rid";
let ridSeq = 0;
function assignRid(row: Row): void {
  const r = row as Record<string, unknown>;
  if (!r[RID]) r[RID] = String(++ridSeq);
}
function rid(row: Row): string {
  return ((row as Record<string, unknown>)[RID] as string) ?? "";
}

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

// --- staged edits -------------------------------------------------------
// Opt-in via the manifest: edits, added rows, and deletions are buffered
// locally (keyed by each row's stable id) so the user reviews them and commits
// or discards as a batch. On commit they replay through the same per-row
// Insert/Update/Delete routes used by the immediate path — no extra contract.
const staged = computed(
  () => Boolean(tableConfig.value?.stagedEdits) && editable.value,
);
// rid -> { field -> original value } for cells changed since the last commit.
const edits = reactive(new Map<string, Map<string, unknown>>());
const insertedRows = reactive(new Set<string>());
const deletedRows = reactive(new Set<string>());
const committing = ref(false);

const pendingCount = computed(() => {
  const ids = new Set<string>();
  for (const id of edits.keys()) ids.add(id);
  for (const id of insertedRows) ids.add(id);
  for (const id of deletedRows) ids.add(id);
  return ids.size;
});

function isInserted(row: Row): boolean {
  return insertedRows.has(rid(row));
}
function isDeleted(row: Row): boolean {
  return deletedRows.has(rid(row));
}
function isEdited(row: Row, field: string): boolean {
  return edits.get(rid(row))?.has(field) ?? false;
}

function clearStaging(): void {
  edits.clear();
  insertedRows.clear();
  deletedRows.clear();
}

function stageCellEdit(row: Row, field: string, prev: unknown): void {
  const id = rid(row);
  if (insertedRows.has(id)) return; // new row: value ships with the insert
  if (!edits.has(id)) edits.set(id, new Map());
  const inner = edits.get(id)!;
  if (!inner.has(field)) inner.set(field, prev);
  if (row[field] === inner.get(field)) {
    inner.delete(field);
    if (inner.size === 0) edits.delete(id);
  }
}

function onDeleteClick(row: Row): void {
  if (!staged.value) {
    askDeleteRow(row);
    return;
  }
  const id = rid(row);
  if (insertedRows.has(id)) {
    rows.value = rows.value.filter((r) => rid(r) !== id);
    insertedRows.delete(id);
    edits.delete(id);
    deletedRows.delete(id);
    return;
  }
  if (deletedRows.has(id)) deletedRows.delete(id);
  else deletedRows.add(id);
}

function canDelete(row: Row): boolean {
  return (
    (Boolean(deleteSource.value) && !!keyFor(row)) ||
    (staged.value && isInserted(row))
  );
}

function insertValues(row: Row): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const col of columns.value) {
    const v = row[col.key];
    if (v !== "" && v !== undefined) values[col.key] = v;
  }
  return values;
}

async function commitStaged(): Promise<void> {
  committing.value = true;
  try {
    for (const row of rows.value) {
      const id = rid(row);
      if (deletedRows.has(id)) continue;
      if (insertedRows.has(id)) {
        if (insertSource.value)
          await mutate(insertSource.value, { values: insertValues(row) });
      } else if (edits.has(id) && updateSource.value) {
        const key = keyFor(row);
        if (!key) continue;
        const values: Record<string, unknown> = {};
        for (const field of edits.get(id)!.keys()) values[field] = row[field];
        await mutate(updateSource.value, { key, values });
      }
    }
    for (const row of rows.value) {
      const id = rid(row);
      if (!deletedRows.has(id) || insertedRows.has(id)) continue;
      const key = keyFor(row);
      if (key && deleteSource.value) await mutate(deleteSource.value, { key });
    }
    clearStaging();
    toast.add({
      severity: "success",
      summary: "Changes committed",
      life: 3000,
    });
    await load(first.value);
  } catch (err) {
    toast.add({
      severity: "error",
      summary: "Commit failed",
      detail: (err as Error).message,
      life: 6000,
    });
  } finally {
    committing.value = false;
  }
}

function discardStaged(): void {
  for (const row of rows.value) {
    const id = rid(row);
    if (insertedRows.has(id)) continue;
    const inner = edits.get(id);
    if (inner) for (const [field, orig] of inner) row[field] = orig;
  }
  rows.value = rows.value.filter((r) => !insertedRows.has(rid(r)));
  clearStaging();
}

// -----------------------------------------------------------------------

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
      detail: "This row has no key, so it cannot be edited.",
      life: 5000,
    });
    return;
  }
  const value = coerce(e.value, e.newValue);
  if (value === e.value) return;
  data[field] = value;
  if (staged.value) {
    stageCellEdit(data, field, e.value);
    return;
  }
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
  for (const col of columns.value) insertDraft.value[col.key] = "";
  showInsert.value = true;
}

async function submitInsert(): Promise<void> {
  const src = insertSource.value;
  if (!src) return;
  const values: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(insertDraft.value)) {
    if (v !== "") values[k] = v;
  }
  if (staged.value) {
    const row = { ...values } as Row;
    assignRid(row);
    rows.value.unshift(row);
    insertedRows.add(rid(row));
    showInsert.value = false;
    return;
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

const hidden = computed(() => {
  const set = new Set(RESERVED);
  for (const key of tableConfig.value?.hiddenColumns ?? []) set.add(key);
  return set;
});

const columns = computed<ColumnSpec[]>(() => {
  if (declaredColumns.value?.length) return declaredColumns.value;
  const sample = rows.value[0];
  if (!sample) return [];
  return Object.keys(sample)
    .filter((k) => !hidden.value.has(k))
    .map((key) => ({ key, label: key }));
});

// Linked cells: a row's `_links` map (column -> related resource ref) makes
// those cells navigation links that open the related resource. Generic — the
// renderer has no notion of what the link means to the plugin.
function linkRef(row: Row, col: ColumnSpec): ResourceRef | null {
  const ref = row._links?.[col.key];
  return ref && row[col.key] != null && row[col.key] !== "" ? ref : null;
}
function openLink(ref: ResourceRef): void {
  emit("select", { ref } as Row);
}

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
  clearStaging();
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
    page.items.forEach(assignRid);
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
  if (staged.value && isDeleted(row)) return "line-through opacity-50";
  if (staged.value && isInserted(row))
    return "bg-emerald-50 dark:bg-emerald-500/10";
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
  if (pendingCount.value > 0) return; // don't clobber buffered staged edits
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

    <div
      v-if="staged && pendingCount"
      class="flex items-center gap-2 border-b border-amber-300 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'git-commit-horizontal' }"
        :size="14"
      />
      <span
        >{{ pendingCount }} unsaved
        {{ pendingCount === 1 ? "change" : "changes" }}</span
      >
      <div class="ml-auto flex gap-2">
        <Button
          type="button"
          label="Discard"
          severity="secondary"
          :disabled="committing"
          @click="discardStaged"
        />
        <Button
          type="button"
          label="Commit"
          :loading="committing"
          :disabled="committing"
          @click="commitStaged"
        />
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
        :data-key="editable ? '__rid' : 'ref.uid'"
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
              :class="
                staged && isEdited(data as Row, col.key)
                  ? 'rounded bg-amber-100 px-1.5 py-0.5 font-medium text-amber-900 dark:bg-amber-500/20 dark:text-amber-100'
                  : undefined
              "
            >
              <button
                v-if="linkRef(data as Row, col)"
                type="button"
                class="inline-flex items-center gap-1 text-primary-600 hover:underline dark:text-primary-400"
                title="Open linked record"
                @click.stop="openLink(linkRef(data as Row, col)!)"
              >
                {{ display(data as Row, col) }}
                <AppIcon
                  :icon="{ type: 'lucide', value: 'arrow-up-right' }"
                  :size="12"
                />
              </button>
              <span
                v-else-if="col.type === 'badge'"
                class="rounded-full bg-surface-100 px-2 py-0.5 text-xs dark:bg-surface-800"
                >{{ display(data as Row, col) }}</span
              >
              <template v-else>{{ display(data as Row, col) }}</template>
            </span>
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
          v-if="editable && (deleteSource || staged)"
          :pt="{ bodyCell: 'w-12 text-right' }"
        >
          <template #body="{ data }">
            <Button
              v-if="canDelete(data as Row)"
              type="button"
              text
              rounded
              :severity="
                staged && isDeleted(data as Row) ? 'secondary' : 'danger'
              "
              :title="
                staged && isDeleted(data as Row) ? 'Undo delete' : 'Delete row'
              "
              :aria-label="
                staged && isDeleted(data as Row) ? 'Undo delete' : 'Delete row'
              "
              @click.stop="onDeleteClick(data as Row)"
            >
              <AppIcon
                :icon="{
                  type: 'lucide',
                  value:
                    staged && isDeleted(data as Row) ? 'rotate-ccw' : 'trash-2',
                }"
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
        root: dialogRoot('max-w-lg'),
      }"
    >
      <div class="flex max-h-[60vh] flex-col gap-3 overflow-auto p-1">
        <label
          v-for="col in columns"
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
        root: dialogRoot('max-w-3xl'),
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
