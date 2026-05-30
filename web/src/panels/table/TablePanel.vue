<script setup lang="ts">
import {
  computed,
  onActivated,
  onDeactivated,
  onUnmounted,
  reactive,
  ref,
  watch as vueWatch,
} from "vue";
import { useDocumentVisibility, useIntervalFn } from "@vueuse/core";
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
  Field,
  FieldType,
  FilterOption,
  ResourceEvent,
  ResourceFilter,
  ResourceRef,
  Row,
  TablePanelConfig,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import { formatBytes } from "../file/fileTypes";
import { dialogRoot, inputClass } from "../../primevue/preset";
import { cn } from "../../utils/cn";
import {
  deleteMutation,
  insertMutation,
  updateMutation,
  type RowMutation,
} from "./mutation";
import RowDetailDialog, { type DetailItem } from "./RowDetailDialog.vue";
import { useNavigableKinds } from "../core/navigable";
import SkeletonList from "../../components/SkeletonList.vue";
import ActionBar from "../shared/ActionBar.vue";
import { badgeClassFor } from "../shared/severity";
import PanelError from "../shared/PanelError.vue";
import FormField from "../form/FormField.vue";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{
  select: [row: Row];
  actionDone: [action: Action, result?: Record<string, unknown>];
}>();

const toast = useToast();

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
  "_id",
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
const selection = ref<Row[]>([]);
const actionOutput = ref<{
  title: string;
  output: string;
  truncated: boolean;
} | null>(null);
const deleteTarget = ref<Row | null>(null);
const deleteBusy = ref(false);
const deleteError = ref<string | null>(null);

const declaredColumns = computed(
  () => (props.config as TablePanelConfig | undefined)?.columns,
);
const tableConfig = computed(
  () => props.config as TablePanelConfig | undefined,
);
const columnsSource = computed(() => tableConfig.value?.columnsSource);

// Manifest-driven toolbar filters (e.g. a namespace selector). The chosen value
// is merged into the list/watch route params so the table scopes server-side.
const filters = computed(() => tableConfig.value?.filters ?? []);
const filterValues = reactive<Record<string, string>>({});
const filterOptions = reactive<Record<string, FilterOption[]>>({});

function activeFilterParams(): Record<string, string> {
  const out: Record<string, string> = {};
  for (const f of filters.value) {
    if (filterValues[f.param]) out[f.param] = filterValues[f.param];
  }
  return out;
}

const effectiveSource = computed<DataSource | undefined>(() => {
  if (!props.source) return undefined;
  const extra = activeFilterParams();
  return Object.keys(extra).length
    ? { ...props.source, params: { ...props.source.params, ...extra } }
    : props.source;
});

const effectiveWatch = computed<DataSource | undefined>(() => {
  const w = tableConfig.value?.watch;
  if (!w) return undefined;
  const extra = activeFilterParams();
  return Object.keys(extra).length
    ? { ...w, params: { ...w.params, ...extra } }
    : w;
});

function filterSelectOptions(f: ResourceFilter): FilterOption[] {
  return [
    { value: "", label: f.allLabel ?? "All" },
    ...(filterOptions[f.key] ?? []),
  ];
}

async function loadFilterOptions(): Promise<void> {
  for (const f of filters.value) {
    if (f.options) {
      filterOptions[f.key] = f.options;
      continue;
    }
    if (!f.optionsSource) continue;
    try {
      const page = await fetchPage<Row>(props.connectionId, f.optionsSource, {
        resource: props.resource,
      });
      const valueField = f.valueField ?? "name";
      const labelField = f.labelField ?? valueField;
      filterOptions[f.key] = page.items
        .map((r) => {
          const row = r as Record<string, unknown>;
          return {
            value: String(row[valueField] ?? ""),
            label: String(row[labelField] ?? row[valueField] ?? ""),
          };
        })
        .filter((o) => o.value);
    } catch {
      filterOptions[f.key] = [];
    }
  }
}

function onFilterChange(f: ResourceFilter, value: string | null): void {
  if (value) filterValues[f.param] = value;
  else delete filterValues[f.param];
  first.value = 0;
  void load(0);
  startWatch();
}
const dynamicColumns = ref<ColumnSpec[]>([]);
const columnsLoading = ref(false);
const actionIds = computed(() => tableConfig.value?.actionIds ?? []);
const rowActionIds = computed(() => tableConfig.value?.rowActionIds ?? []);
const globalActions = computed(() => resolveActions(actionIds.value));
const rowActions = computed(() => resolveActions(rowActionIds.value));
const emptyText = computed(() => tableConfig.value?.emptyText ?? "No rows.");
const DEFAULT_COLUMN_WIDTH = "16rem";
const TYPE_COLUMN_WIDTH: Partial<
  Record<NonNullable<ColumnSpec["type"]>, string>
> = {
  badge: "10rem",
  bool: "8rem",
  bytes: "9rem",
  datetime: "14rem",
  number: "9rem",
  json: "22rem",
};

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
// Row selection (checkboxes) is offered when a table declares row actions and
// isn't an inline-editable grid (which has its own row controls).
const selectable = computed(
  () => rowActions.value.length > 0 && !editable.value,
);
const selectedRefs = computed(() =>
  selection.value.map((r) => r.ref).filter((r): r is ResourceRef => Boolean(r)),
);
const addRowLoading = computed(
  () => columnsLoading.value || (loading.value && !columns.value.length),
);
const addRowTitle = computed(() => {
  if (columns.value.length) return "Add a row";
  return "No editable columns available";
});

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
          await mutate(insertSource.value, insertMutation(insertValues(row)));
      } else if (edits.has(id) && updateSource.value) {
        const key = keyFor(row);
        if (!key) continue;
        const values: Record<string, unknown> = {};
        for (const field of edits.get(id)!.keys()) values[field] = row[field];
        await mutate(updateSource.value, updateMutation(key, values));
      }
    }
    for (const row of rows.value) {
      const id = rid(row);
      if (!deletedRows.has(id) || insertedRows.has(id)) continue;
      const key = keyFor(row);
      if (key && deleteSource.value)
        await mutate(deleteSource.value, deleteMutation(key));
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

async function mutate(src: DataSource, body: RowMutation): Promise<void> {
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
    await mutate(src, updateMutation(key, { [field]: value }));
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
  deleteTarget.value = row;
  deleteError.value = null;
}

function closeDeleteDialog(): void {
  if (deleteBusy.value) return;
  deleteTarget.value = null;
  deleteError.value = null;
}

const deleteRowLabel = computed(() => {
  const row = deleteTarget.value;
  if (!row) return "";
  const raw = row.label ?? row.name ?? row.id ?? row._key;
  if (raw == null) return "";
  if (typeof raw === "string" || typeof raw === "number") return String(raw);
  return "";
});

async function confirmDeleteRow(): Promise<void> {
  const src = deleteSource.value;
  const row = deleteTarget.value;
  const key = row ? keyFor(row) : null;
  if (!src || !key) {
    closeDeleteDialog();
    return;
  }
  deleteBusy.value = true;
  deleteError.value = null;
  try {
    await mutate(src, deleteMutation(key));
    toast.add({ severity: "success", summary: "Row deleted", life: 3000 });
    deleteTarget.value = null;
    await load(first.value);
  } catch (err) {
    deleteError.value = (err as Error).message;
    toast.add({
      severity: "error",
      summary: "Delete failed",
      detail: (err as Error).message,
      life: 6000,
    });
  } finally {
    deleteBusy.value = false;
  }
}

const showInsert = ref(false);
const insertDraft = ref<Record<string, unknown>>({});
const inserting = ref(false);

// The add-row form derives each input widget from its column's type, so a number
// column gets a number input, a boolean a toggle, JSON a code area, etc.
const COLUMN_FIELD_TYPE: Partial<
  Record<NonNullable<ColumnSpec["type"]>, FieldType>
> = {
  number: "number",
  bool: "toggle",
  json: "json",
};
const insertFields = computed<Field[]>(() =>
  columns.value.map((col) => ({
    key: col.key,
    label: col.label,
    type: COLUMN_FIELD_TYPE[col.type ?? "text"] ?? "text",
    placeholder: col.nullable ? "NULL" : undefined,
  })),
);

function openInsert(): void {
  insertDraft.value = {};
  showInsert.value = true;
}

async function submitInsert(): Promise<void> {
  const src = insertSource.value;
  if (!src) return;
  const values: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(insertDraft.value)) {
    if (v !== "" && v !== undefined && v !== null) values[k] = v;
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
    await mutate(src, insertMutation(values));
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
  if (dynamicColumns.value.length) return dynamicColumns.value;
  const sample = rows.value[0];
  if (!sample) return [];
  return Object.keys(sample)
    .filter((k) => !hidden.value.has(k))
    .map((key) => ({ key, label: key }));
});

function dynamicColumnKey(row: Row): string {
  return String(row.name ?? row.column_name ?? row.column ?? row.key ?? "");
}

function dynamicColumnLabel(row: Row, key: string): string {
  return String(row.label ?? row.name ?? row.column_name ?? row.column ?? key);
}

// Maps a column's declared data type (a DB type string like "integer" or
// "timestamptz", or a generic column type) to a renderer type, so a runtime
// column grid and its add-row form pick the right widget without per-plugin code.
function mapColumnType(raw: unknown): ColumnSpec["type"] | undefined {
  const t = String(raw ?? "").toLowerCase();
  if (!t) return undefined;
  if (/bool/.test(t)) return "bool";
  if (/json/.test(t)) return "json";
  if (/(int|serial|numeric|decimal|real|double|float|money|number)/.test(t))
    return "number";
  if (/(date|time|timestamp)/.test(t)) return "datetime";
  return undefined;
}

function dynamicColumn(row: Row): ColumnSpec | null {
  const key = dynamicColumnKey(row);
  if (!key || hidden.value.has(key)) return null;
  return {
    key,
    label: dynamicColumnLabel(row, key),
    type: mapColumnType((row as Record<string, unknown>).type),
    nullable: row.nullable === true,
  };
}

async function loadDynamicColumns(): Promise<void> {
  dynamicColumns.value = [];
  if (declaredColumns.value?.length || !columnsSource.value) return;
  columnsLoading.value = true;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      columnsSource.value,
      { resource: props.resource },
      { limit: 500 },
    );
    dynamicColumns.value = page.items
      .map(dynamicColumn)
      .filter((col): col is ColumnSpec => Boolean(col));
  } finally {
    columnsLoading.value = false;
  }
}

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

function formatNumber(v: number, col: ColumnSpec): string {
  const n = col.precision != null ? v.toFixed(col.precision) : String(v);
  return col.type === "percent" ? `${n}%` : n;
}

function display(row: Row, col: ColumnSpec): string {
  const v = row[col.key];
  if (v === undefined || v === null || v === "") return "—";
  if (col.type === "bytes" && typeof v === "number") return formatBytes(v);
  if (
    (col.type === "number" || col.type === "percent") &&
    typeof v === "number"
  )
    return formatNumber(v, col);
  if (col.type === "datetime" && typeof v === "string")
    return new Date(v).toLocaleString();
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

function badgeClass(row: Row, col: ColumnSpec): string {
  return badgeClassFor(col.severities, row[col.key]);
}

function columnWidth(col: ColumnSpec): string {
  return (
    col.width || TYPE_COLUMN_WIDTH[col.type ?? "text"] || DEFAULT_COLUMN_WIDTH
  );
}

function columnStyle(col: ColumnSpec): Record<string, string> {
  const width = columnWidth(col);
  return {
    minWidth: col.width ? width : "7.5rem",
    width,
    maxWidth: width,
  };
}

function cellClass(row: Row, col: ColumnSpec): string {
  const base = "block min-w-0 truncate";
  if (staged.value && isEdited(row, col.key)) {
    return cn(
      base,
      "rounded bg-amber-100 px-1.5 py-0.5 font-medium text-amber-900 dark:bg-amber-500/20 dark:text-amber-100",
    );
  }
  return base;
}

async function load(targetFirst = first.value): Promise<void> {
  if (!effectiveSource.value) return;
  loading.value = true;
  error.value = null;
  selection.value = [];
  clearStaging();
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      effectiveSource.value,
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
    await loadDynamicColumns();
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

function isInteractiveTarget(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    Boolean(
      target.closest(
        'button,a,input,select,textarea,[role="button"],[role="checkbox"]',
      ),
    )
  );
}

// Row-click is automatic: a row whose ref is a navigable resource opens it,
// everything else selects (when selectable). `RowClick` only overrides this —
// `detail` for the field dialog, or an explicit navigate/select/none.
const navigableKinds = useNavigableKinds();
const rowClickMode = computed(() => tableConfig.value?.rowClick);
const detailEnabled = computed(() => rowClickMode.value === "detail");
const detailRow = ref<Row | null>(null);

function navigates(row: Row): boolean {
  return Boolean(row.ref && navigableKinds.value.has(row.ref.kind));
}

function toggleSelection(row: Row): void {
  selection.value =
    selection.value.length === 1 && rid(selection.value[0]) === rid(row)
      ? []
      : [row];
}

// Stable row identity for keying/diff/refresh: a behavior-free `_id`, or a
// navigable `ref.uid`; the client-only `__rid` covers editable/selectable grids.
const dataKeyField = computed(() => {
  if (editable.value || selectable.value) return "__rid";
  const r = rows.value[0] as (Row & { _id?: unknown }) | undefined;
  if (r?.ref?.uid) return "ref.uid";
  if (r?._id != null) return "_id";
  return "__rid";
});

function onRowClick(e: DataTableRowClickEvent): void {
  const row = e.data as Row;
  if (isInteractiveTarget(e.originalEvent?.target ?? null)) return;
  if (editable.value) return; // body reserved for cell editing
  switch (rowClickMode.value) {
    case "none":
      return;
    case "detail":
      detailRow.value = row;
      return;
    case "select":
      toggleSelection(row);
      return;
    case "navigate":
      if (row.ref) emit("select", row);
      return;
  }
  // Default: navigate if the row is a navigable resource (or nothing else can
  // handle the click); otherwise select. Selection is also available via the
  // checkbox column.
  if (navigates(row) || (row.ref && !selectable.value)) emit("select", row);
  else if (selectable.value) toggleSelection(row);
}

function rowClickable(row: Row): boolean {
  if (editable.value) return false;
  const mode = rowClickMode.value;
  if (mode) return mode !== "none";
  return navigates(row) || Boolean(row.ref) || selectable.value;
}

function rowClass(row: Row): string {
  if (staged.value && isDeleted(row)) return "line-through opacity-50";
  if (staged.value && isInserted(row))
    return "bg-emerald-50 dark:bg-emerald-500/10";
  return rowClickable(row) ? "cursor-pointer" : "";
}

function humanize(key: string): string {
  const spaced = key
    .replace(/[_-]+/g, " ")
    .replace(/([a-z\d])([A-Z])/g, "$1 $2");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

const detailItems = computed<DetailItem[]>(() => {
  const r = detailRow.value;
  if (!r) return [];
  const items: DetailItem[] = [];
  const declared = new Set<string>();
  for (const col of columns.value) {
    declared.add(col.key);
    items.push({
      key: col.key,
      label: col.label,
      text: display(r, col),
      badge: col.type === "badge" ? badgeClass(r, col) : undefined,
    });
  }
  for (const key of Object.keys(r)) {
    if (declared.has(key) || hidden.value.has(key)) continue;
    const v = (r as Record<string, unknown>)[key];
    if (v === undefined || v === null || v === "") continue;
    items.push({
      key,
      label: humanize(key),
      text: typeof v === "object" ? JSON.stringify(v) : String(v),
    });
  }
  return items;
});

const detailTitle = computed(() => {
  const r = detailRow.value;
  if (!r) return "";
  const raw =
    r.label ?? r.name ?? r.ref?.name ?? r[columns.value[0]?.key ?? ""];
  return raw != null && raw !== "" ? String(raw) : "Details";
});

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

// Watch events arrive one WS frame at a time; we buffer a burst and apply it in
// a single pass per frame so a flood becomes one reactive update, not N.
let pendingEvents: ResourceEvent[] = [];
let flushHandle: number | undefined;

function applyEvent(ev: ResourceEvent): void {
  if (pendingCount.value > 0) return; // don't clobber buffered staged edits
  pendingEvents.push(ev);
  if (flushHandle === undefined)
    flushHandle = requestAnimationFrame(flushEvents);
}

function flushEvents(): void {
  flushHandle = undefined;
  const batch = pendingEvents;
  pendingEvents = [];
  if (!batch.length || pendingCount.value > 0) return;
  const index = new Map<string, number>();
  rows.value.forEach((r, i) => {
    if (r.ref?.uid) index.set(r.ref.uid, i);
  });
  const next = rows.value.slice();
  const additions = new Map<string, Row>();
  const removed = new Set<number>();
  for (const ev of batch) {
    const uid = ev.ref.uid;
    const idx = index.get(uid);
    // Tolerate any casing a plugin sends (the contract is lowercase).
    const type = String(ev.type).toLowerCase();
    if (type === "deleted") {
      if (idx !== undefined) removed.add(idx);
      additions.delete(uid);
    } else if (idx !== undefined) {
      removed.delete(idx);
      if (ev.resource) next[idx] = { ...next[idx], ...(ev.resource as Row) };
    } else if (additions.has(uid)) {
      if (ev.resource)
        additions.set(uid, { ...additions.get(uid)!, ...(ev.resource as Row) });
    } else if ((type === "added" || type === "updated") && ev.resource) {
      additions.set(uid, { ...(ev.resource as Row), ref: ev.ref });
    }
  }
  const kept = removed.size ? next.filter((_, i) => !removed.has(i)) : next;
  rows.value = additions.size ? [...additions.values(), ...kept] : kept;
}

let stopWatch: (() => void) | undefined;
function startWatch(): void {
  stopWatch?.();
  // A live table uses either the interval poll or the watch socket, never both.
  const ds = refreshMs.value > 0 ? undefined : effectiveWatch.value;
  stopWatch = ds
    ? watchResource(
        props.connectionId,
        ds,
        { resource: props.resource },
        applyEvent,
      )
    : undefined;
}

// Live poll: re-fetch the current page in place, leaving loading/selection/
// staged state untouched so the view never flickers or loses the user's place.
const refreshMs = computed(() => tableConfig.value?.refreshIntervalMs ?? 0);
const visibility = useDocumentVisibility();
// Under KeepAlive an off-screen tab stays mounted; pause its poll so a plugin
// with many live tabs only refreshes the visible one. No-op when not kept alive.
const active = ref(true);
onActivated(() => (active.value = true));
onDeactivated(() => (active.value = false));

async function refresh(): Promise<void> {
  if (!effectiveSource.value || loading.value || committing.value) return;
  if (pendingCount.value > 0) return;
  if (showInsert.value || deleteTarget.value || actionOutput.value) return;
  if (detailRow.value) return;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      effectiveSource.value,
      { resource: props.resource },
      {
        cursor: first.value > 0 ? String(first.value) : "",
        limit: pageSize.value,
        filter: filterText.value ? { q: filterText.value } : undefined,
        sort: sortField.value
          ? [{ field: sortField.value, desc: sortOrder.value === -1 }]
          : undefined,
      },
    );
    page.items.forEach(assignRid);
    const keep = new Set(selectedRefs.value.map((r) => r.uid));
    rows.value = page.items;
    if (keep.size)
      selection.value = page.items.filter(
        (r) => r.ref?.uid && keep.has(r.ref.uid),
      );
    total.value = page.total;
  } catch {
    // transient failure: keep the current rows rather than blanking the table
  }
}

const { pause: pausePoll, resume: resumePoll } = useIntervalFn(
  refresh,
  () => refreshMs.value || 1000,
  { immediate: false },
);
vueWatch(
  () => refreshMs.value > 0 && active.value && visibility.value === "visible",
  (on, was) => {
    if (!on) {
      pausePoll();
      return;
    }
    if (was === false) void refresh(); // catch up after being paused
    resumePoll();
  },
  { immediate: true },
);

function applyDefaultSort(): void {
  const ds = tableConfig.value?.defaultSort;
  sortField.value = ds?.field;
  sortOrder.value = ds ? (ds.desc ? -1 : 1) : undefined;
}

vueWatch(
  () => [props.connectionId, props.source?.routeId, props.resource?.uid],
  () => {
    filterText.value = "";
    for (const k of Object.keys(filterValues)) delete filterValues[k];
    applyDefaultSort();
    first.value = 0;
    void loadFilterOptions();
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
  if (flushHandle !== undefined) cancelAnimationFrame(flushHandle);
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
      <div v-for="f in filters" :key="f.key" class="w-44">
        <Select
          :model-value="filterValues[f.param] ?? ''"
          :options="filterSelectOptions(f)"
          option-label="label"
          option-value="value"
          :aria-label="f.label"
          @update:model-value="onFilterChange(f, $event)"
        />
      </div>
      <span v-if="total != null" class="text-xs text-surface-400"
        >{{ total }} total</span
      >
      <Button
        v-if="editable && insertSource"
        type="button"
        severity="secondary"
        :disabled="loading || addRowLoading || !columns.length"
        :title="addRowTitle"
        @click="openInsert"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'plus' }"
          :size="14"
          :loading="addRowLoading"
        />
        Add row
      </Button>
      <!-- List-level actions inherit the list's own params as their scope, so an
           action declared on a list runs within that list's context for free. -->
      <ActionBar
        v-if="globalActions.length"
        :connection-id="connectionId"
        :actions="globalActions"
        :resource="resource"
        :scope="source?.params"
        @done="onActionDone"
      />
      <template v-if="rowActions.length && selection.length">
        <span class="text-xs text-surface-400"
          >{{ selection.length }} selected</span
        >
        <ActionBar
          :connection-id="connectionId"
          :actions="rowActions"
          :resource="selection.length === 1 ? selectedRefs[0] : null"
          :record="selection.length === 1 ? selection[0] : null"
          :resources="selectedRefs"
          :records="selection"
          @done="onActionDone"
        />
      </template>
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
        v-model:selection="selection"
        :value="rows"
        :data-key="dataKeyField"
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
          v-if="selectable"
          selection-mode="multiple"
          :header-style="{ width: '3rem' }"
          :body-style="{ width: '3rem' }"
        />
        <Column
          v-for="col in columns"
          :key="col.key"
          :field="col.key"
          :header="col.label"
          :sortable="col.sortable"
          :style="columnStyle(col)"
          :header-style="columnStyle(col)"
          :body-style="columnStyle(col)"
        >
          <template #body="{ data }">
            <span
              data-test="table-cell-value"
              :class="cellClass(data as Row, col)"
              :style="{ maxWidth: columnWidth(col) }"
              :title="display(data as Row, col)"
            >
              <button
                v-if="linkRef(data as Row, col)"
                type="button"
                class="inline-flex max-w-full items-center gap-1 text-primary-600 hover:underline dark:text-primary-400"
                :title="display(data as Row, col)"
                @click.stop="openLink(linkRef(data as Row, col)!)"
              >
                <span class="truncate">{{ display(data as Row, col) }}</span>
                <AppIcon
                  :icon="{ type: 'lucide', value: 'arrow-up-right' }"
                  :size="12"
                />
              </button>
              <span
                v-else-if="col.type === 'badge'"
                class="inline-block max-w-full truncate rounded-full px-2 py-0.5 align-bottom text-xs"
                :class="badgeClass(data as Row, col)"
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
        <Column
          v-if="detailEnabled && !editable"
          :header-style="{ width: '3rem' }"
          :pt="{ bodyCell: 'w-12 text-right' }"
        >
          <template #body="{ data }">
            <Button
              type="button"
              text
              rounded
              severity="secondary"
              title="View details"
              aria-label="View details"
              @click.stop="detailRow = data as Row"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'panel-right-open' }"
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
        <FormField
          v-for="f in insertFields"
          :key="f.key"
          :field="f"
          :model-value="insertDraft[f.key]"
          @update:model-value="insertDraft[f.key] = $event"
        />
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
      :visible="!!deleteTarget"
      modal
      header="Delete row"
      :dismissable-mask="!deleteBusy"
      :closable="!deleteBusy"
      :pt="{ root: dialogRoot('max-w-md') }"
      @update:visible="(v) => !v && closeDeleteDialog()"
    >
      <div class="flex items-start gap-3">
        <div
          class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-rose-500/10 text-rose-500"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'trash-2' }" :size="18" />
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-surface-900 dark:text-surface-50">
            Delete this row?
          </p>
          <p class="mt-1 text-sm text-surface-500 dark:text-surface-400">
            This change is permanent and cannot be undone.
          </p>
          <p
            v-if="deleteRowLabel"
            class="mt-3 truncate rounded-md border border-surface-200 bg-surface-50 px-2 py-1.5 font-mono text-xs text-surface-600 dark:border-surface-800 dark:bg-surface-900 dark:text-surface-300"
            :title="deleteRowLabel"
          >
            {{ deleteRowLabel }}
          </p>
          <p v-if="deleteError" class="mt-3 text-sm text-red-500">
            {{ deleteError }}
          </p>
        </div>
      </div>
      <template #footer>
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          :disabled="deleteBusy"
          @click="closeDeleteDialog"
        />
        <Button
          type="button"
          label="Delete"
          severity="danger"
          :loading="deleteBusy"
          :disabled="deleteBusy"
          autofocus
          @click="confirmDeleteRow"
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

    <RowDetailDialog
      :visible="!!detailRow"
      :title="detailTitle"
      :items="detailItems"
      @update:visible="(v) => !v && (detailRow = null)"
    />
  </div>
</template>
