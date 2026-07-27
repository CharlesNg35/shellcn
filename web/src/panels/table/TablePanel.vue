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
  type DataTableCellEditInitEvent,
  type DataTablePageEvent,
  type DataTableSortEvent,
  type DataTableRowClickEvent,
} from "primevue/datatable";
import Column from "primevue/column";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import Textarea from "primevue/textarea";
import Select from "primevue/select";
import Menu from "primevue/menu";
import ToggleSwitch from "primevue/toggleswitch";
import { useToast } from "primevue/usetoast";
import { exportRecords, type ExportFormat } from "../shared/exportData";
import { fetchPage, runAction, watch as watchResource } from "@/api/dataSource";
import type {
  Action,
  Column as ColumnSpec,
  DataSource,
  Field,
  FieldType as FieldTypeValue,
  Icon,
  Page,
  ResourceEvent,
  ResourceIdentity,
  Row,
  TablePanelConfig,
} from "@/types/projection";
import {
  ColumnEditor,
  ColumnType,
  FieldType,
  RowClickAction,
} from "@/types/projection";
import type { PanelProps } from "../core/types";
import { formatBytes } from "../file/fileTypes";
import { formatDurationSeconds } from "../specialized/objectDetailFormat";
import { dialogRoot, inputClass } from "@/primevue/preset";
import { cn } from "@/utils/cn";
import {
  deleteMutation,
  insertMutation,
  updateMutation,
  type RowMutation,
} from "./mutation";
import RowDetailDialog, { type DetailItem } from "./RowDetailDialog.vue";
import JsonCellDialog from "./JsonCellDialog.vue";
import {
  cellValueEquals,
  coerceCellValue,
  defaultColumnEditor,
  defaultColumnType,
  fullCellText,
  isCellEditable,
  isInlineEditor,
  isJsonEditor,
  isStructuredValue,
  jsonEditorText,
  stableStringify,
  structuredSummary,
  writableColumns,
} from "./cellEditing";
import { useNavigableKinds } from "../core/navigable";
import { useWorkspaceStore } from "@/stores/workspace";
import SkeletonList from "@/components/SkeletonList.vue";
import ActionBar from "../shared/ActionBar.vue";
import { badgeClassFor } from "../shared/severity";
import PanelError from "../shared/PanelError.vue";
import FormField from "../form/FormField.vue";
import AppIcon from "@/components/AppIcon.vue";
import { useDirtyGuard } from "../shared/useDirtyGuard";
import { useConnectionInvalidationRefresh } from "../shared/useConnectionInvalidationRefresh";

const props = defineProps<PanelProps>();
const emit = defineEmits<{
  select: [row: Row];
  actionDone: [action: Action, result?: Record<string, unknown>];
}>();

const toast = useToast();
const workspace = useWorkspaceStore();

// Envelope keys the panel owns; never renderable as data.
const INTERNAL = ["_key", "_links", "_id", "__rid"];
// Tree/projection vocabulary — only ambiguous when columns are inferred from a row.
const RESERVED = new Set([
  "key",
  "label",
  "leaf",
  "ref",
  "childrenSource",
  "badge",
  ...INTERNAL,
]);

// One formatter for every datetime cell; `toLocaleString()` builds a new
// Intl.DateTimeFormat on each call and these run per cell per render.
const DATE_FMT = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "numeric",
  day: "numeric",
  hour: "numeric",
  minute: "numeric",
  second: "numeric",
});

const RID = "__rid";
let ridSeq = 0;

// Row identity must survive a refetch: PrimeVue keys every <tr> (and its editing
// meta) on it, so a fresh sequence number per fetch would remount the whole grid.
function rowIdentity(row: Row): string | undefined {
  if (row.ref?.uid) return row.ref.uid;
  const id = (row as Record<string, unknown>)._id;
  if (id != null && id !== "") return String(id);
  if (row._key && Object.keys(row._key).length)
    return stableStringify(row._key);
  const cols = tableConfig.value?.rowKey;
  if (cols?.length && cols.every((c) => row[c] != null))
    return stableStringify(Object.fromEntries(cols.map((c) => [c, row[c]])));
  return undefined;
}

function assignRid(row: Row): void {
  const r = row as Record<string, unknown>;
  if (r[RID]) return;
  const identity = rowIdentity(row);
  r[RID] = identity ? `k:${identity}` : String(++ridSeq);
}
function rid(row: Row): string {
  return ((row as Record<string, unknown>)[RID] as string) ?? "";
}

const rows = ref<Row[]>([]);
const total = ref<number | undefined>();
const hasMore = ref(false);
const loading = ref(false);
const refreshing = ref(false);
const editingRid = ref<string | null>(null);
let loadSeq = 0;
const error = ref<string | null>(null);
const filterText = ref("");
const sortField = ref<string | undefined>();
const sortOrder = ref<number | undefined>();
const first = ref(0);
const pageSize = ref(50);
const cursorsByFirst = reactive(new Map<number, string>());
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

const stateKey = computed(() =>
  [
    props.connectionId,
    props.source?.routeId ?? "",
    stableStringify(props.source?.params ?? {}),
    props.resource?.uid ?? "",
  ].join("|"),
);

function defaultSortState(): { sortField?: string; sortOrder?: number } {
  const ds = tableConfig.value?.defaultSort;
  return {
    sortField: ds?.field,
    sortOrder: ds ? (ds.desc ? -1 : 1) : undefined,
  };
}

function restoreTableState(): void {
  const defaults = defaultSortState();
  const state = workspace.tableState(stateKey.value, {
    filterText: "",
    sortField: defaults.sortField,
    sortOrder: defaults.sortOrder,
    first: 0,
    pageSize: 50,
  });
  filterText.value = state.filterText;
  sortField.value = state.sortField;
  sortOrder.value = state.sortOrder;
  first.value = state.first;
  pageSize.value = state.pageSize;
}

function saveTableState(): void {
  if (!stateKey.value) return;
  workspace.setTableState(stateKey.value, {
    filterText: filterText.value,
    sortField: sortField.value,
    sortOrder: sortOrder.value,
    first: first.value,
    pageSize: pageSize.value,
  });
}

function resetCursors(): void {
  cursorsByFirst.clear();
}

function cursorFor(targetFirst: number): string {
  if (targetFirst <= 0) return "";
  return cursorsByFirst.get(targetFirst) ?? String(targetFirst);
}

// The paginator advances by pageSize, so that offset must map to this cursor even
// when the source returned a short page; keep the item-count offset too for
// sources that page by however many rows they actually produced.
function rememberNextCursor(targetFirst: number, page: Page<Row>): void {
  if (!page.nextCursor) return;
  cursorsByFirst.set(targetFirst + pageSize.value, page.nextCursor);
  if (page.items.length)
    cursorsByFirst.set(targetFirst + page.items.length, page.nextCursor);
}

const watchSource = computed(() => tableConfig.value?.watch);
const dynamicColumns = ref<ColumnSpec[]>([]);
const columnsLoading = ref(false);
const columnsLoaded = ref(false);
const actionIds = computed(() => tableConfig.value?.actionIds ?? []);
const rowActionIds = computed(() => tableConfig.value?.rowActionIds ?? []);
const globalActions = computed(() => resolveActions(actionIds.value));
const rowActions = computed(() => resolveActions(rowActionIds.value));
const emptyText = computed(() => tableConfig.value?.emptyText ?? "No rows.");
const DEFAULT_COLUMN_WIDTH = "16rem";
const TYPE_COLUMN_WIDTH: Partial<
  Record<NonNullable<ColumnSpec["type"]>, string>
> = {
  [ColumnType.Badge]: "10rem",
  [ColumnType.Bool]: "8rem",
  [ColumnType.Bytes]: "9rem",
  [ColumnType.DateTime]: "14rem",
  [ColumnType.Duration]: "9rem",
  [ColumnType.Icon]: "3rem",
  [ColumnType.Number]: "9rem",
  [ColumnType.Json]: "22rem",
  [ColumnType.RelativeTime]: "9rem",
};

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
const exportRange = computed(() =>
  rows.value.length
    ? `rows ${first.value + 1}–${first.value + rows.value.length}`
    : "rows",
);
const exportItems = computed(() => [
  {
    label: `Export ${exportRange.value} as CSV`,
    command: () => runExport("csv"),
  },
  {
    label: `Export ${exportRange.value} as JSON`,
    command: () => runExport("json"),
  },
]);

const insertSource = computed(() => tableConfig.value?.insert);
const updateSource = computed(() => tableConfig.value?.update);
const deleteSource = computed(() => tableConfig.value?.delete);
const editable = computed(
  () =>
    Boolean(tableConfig.value?.editable) &&
    Boolean(insertSource.value || updateSource.value || deleteSource.value),
);
const editableCells = computed(() => editable.value && !!updateSource.value);
function canEditCell(col: ColumnSpec): boolean {
  return editableCells.value && isCellEditable(col);
}
function canInlineEditCell(col: ColumnSpec): boolean {
  return editableCells.value && isInlineEditor(col);
}
function canJsonEditCell(col: ColumnSpec): boolean {
  return editableCells.value && isJsonEditor(col);
}
const selectable = computed(
  () => rowActions.value.length > 0 || tableConfig.value?.selectable === true,
);
const addRowLoading = computed(
  () => columnsLoading.value || (loading.value && !columns.value.length),
);
const addRowTitle = computed(() => {
  if (columns.value.length) return "Add a row";
  return "No editable columns available";
});

const staged = computed(
  () => Boolean(tableConfig.value?.stagedEdits) && editable.value,
);
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
const { confirmBeforeDiscard } = useDirtyGuard({
  isDirty: () => pendingCount.value > 0,
  header: "Discard table changes?",
  message: "This table has unsaved changes. Discard them and continue?",
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
  if (cellValueEquals(row[field], inner.get(field))) {
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

// A field left blank in the insert dialog must be omitted so the column default
// applies; only a materialised row the user explicitly set to NULL ships one.
function insertValues(
  row: Row,
  opts: { keepNull?: boolean } = {},
): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const col of writableColumns(columns.value)) {
    const v = row[col.key];
    if (v === "" || v === undefined) continue;
    if (v === null && !opts.keepNull) continue;
    values[col.key] = v;
  }
  return values;
}

const UNKEYED = "This row has no key, so it cannot be saved.";

// Each row commits independently: a mid-run failure must leave the rows that did
// land un-staged, so pressing Commit again never replays an applied mutation.
async function commitStaged(): Promise<void> {
  committing.value = true;
  const pending = pendingCount.value;
  const failures: Error[] = [];
  const removedIds = new Set<string>();
  let done = 0;
  try {
    for (const row of rows.value) {
      const id = rid(row);
      if (deletedRows.has(id)) continue;
      let body: RowMutation | undefined;
      let src: DataSource | undefined;
      if (insertedRows.has(id)) {
        src = insertSource.value;
        body = insertMutation(insertValues(row, { keepNull: true }));
      } else if (edits.has(id) && updateSource.value) {
        const key = keyFor(row);
        if (!key) {
          failures.push(new Error(UNKEYED));
          continue;
        }
        const values: Record<string, unknown> = {};
        for (const field of edits.get(id)!.keys()) values[field] = row[field];
        src = updateSource.value;
        body = updateMutation(key, values);
      }
      if (!src || !body) continue;
      try {
        await mutate(src, body, row);
        insertedRows.delete(id);
        edits.delete(id);
        done += 1;
      } catch (err) {
        failures.push(err as Error);
      }
    }
    for (const row of rows.value) {
      const id = rid(row);
      if (!deletedRows.has(id) || insertedRows.has(id)) continue;
      if (!deleteSource.value) continue;
      const key = keyFor(row);
      if (!key) {
        failures.push(new Error(UNKEYED));
        continue;
      }
      try {
        await mutate(deleteSource.value, deleteMutation(key), row);
        deletedRows.delete(id);
        edits.delete(id);
        removedIds.add(id);
        done += 1;
      } catch (err) {
        failures.push(err as Error);
      }
    }
    // Deletes that landed are un-staged, so the grid has to drop them here: the
    // reconciling reload only runs when nothing is left pending.
    if (removedIds.size)
      rows.value = rows.value.filter((r) => !removedIds.has(rid(r)));
    if (!failures.length) clearStaging();
    toast.add({
      severity: failures.length ? "warn" : "success",
      summary: `${done} of ${pending} changes committed`,
      detail: failures[0]?.message,
      life: failures.length ? 6000 : 3000,
    });
  } finally {
    committing.value = false;
    if (!pendingCount.value) await load(first.value);
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

// An incomplete key must be null, not a partially-filled object: JSON drops the
// undefined members and the server would see an unfiltered predicate.
function keyFor(row: Row): Record<string, unknown> | null {
  const explicit = row._key;
  if (
    explicit &&
    typeof explicit === "object" &&
    Object.keys(explicit).length
  ) {
    return explicit as Record<string, unknown>;
  }
  const cols = tableConfig.value?.rowKey;
  if (cols?.length) {
    const key: Record<string, unknown> = {};
    for (const c of cols) {
      if (row[c] === undefined || row[c] === null) return null;
      key[c] = row[c];
    }
    return key;
  }
  return null;
}

async function mutate(
  src: DataSource,
  body: RowMutation,
  record?: Row | null,
): Promise<void> {
  await runAction(
    props.connectionId,
    src.routeId,
    { resource: props.resource, record },
    body,
    src.params ?? {},
    src.method ?? "POST",
  );
}

async function onCellEditComplete(
  e: DataTableCellEditCompleteEvent,
): Promise<void> {
  try {
    const src = updateSource.value;
    if (!src) return;
    const data = e.data as Row;
    const field = e.field;
    const col = columns.value.find((c) => c.key === field);
    if (!col || !canInlineEditCell(col)) return;
    const value = coerceCellValue(col, e.value, e.newValue);
    await commitCellValue(data, col, e.value, value);
  } finally {
    editingRid.value = null;
  }
}

function onCellEditInit(e: DataTableCellEditInitEvent): void {
  editingRid.value = rid(e.data as Row) || null;
}

async function commitCellValue(
  data: Row,
  col: ColumnSpec,
  prev: unknown,
  value: unknown,
): Promise<boolean> {
  const src = updateSource.value;
  if (!src) return false;
  const field = col.key;
  if (staged.value && insertedRows.has(rid(data))) {
    // Not persisted yet — the value ships with the insert at commit time.
    if (!cellValueEquals(value, prev)) data[field] = value;
    return true;
  }
  const key = keyFor(data);
  if (!key) {
    data[field] = prev;
    toast.add({
      severity: "warn",
      summary: "Read-only row",
      detail: "This row has no key, so it cannot be edited.",
      life: 5000,
    });
    return false;
  }
  if (cellValueEquals(value, prev)) return true;
  data[field] = value;
  if (staged.value) {
    stageCellEdit(data, field, prev);
    return true;
  }
  try {
    await mutate(src, updateMutation(key, { [field]: value }), data);
  } catch (err) {
    data[field] = prev;
    toast.add({
      severity: "error",
      summary: "Update failed",
      detail: (err as Error).message,
      life: 6000,
    });
    return false;
  }
  // Editing a key column invalidates the row's key envelope: refetch so the next
  // edit on that row does not target the old key.
  if (field in key) await load(first.value);
  return true;
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
    await mutate(src, deleteMutation(key), row);
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

const COLUMN_FIELD_TYPE: Partial<
  Record<NonNullable<ColumnSpec["editor"]>, FieldTypeValue>
> = {
  [ColumnEditor.Text]: FieldType.Text,
  [ColumnEditor.Textarea]: FieldType.Textarea,
  [ColumnEditor.Number]: FieldType.Number,
  [ColumnEditor.Toggle]: FieldType.Toggle,
  [ColumnEditor.Select]: FieldType.Select,
  [ColumnEditor.Json]: FieldType.Json,
};
const insertFields = computed<Field[]>(() =>
  writableColumns(columns.value).map((col) => ({
    key: col.key,
    label: col.label,
    type: COLUMN_FIELD_TYPE[col.editor ?? ColumnEditor.Text] ?? FieldType.Text,
    options: col.options,
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
  const values = insertValues(insertDraft.value as Row);
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
    await mutate(src, insertMutation(values), values as Row);
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

const hidden = computed(() => {
  const set = new Set(RESERVED);
  for (const key of tableConfig.value?.hiddenColumns ?? []) set.add(key);
  return set;
});

// Declared columns name real data, so only the envelope keys and the manifest's
// own hiddenColumns may suppress them.
const userHidden = computed(() => {
  const set = new Set(INTERNAL);
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

function dynamicColumn(row: Row): ColumnSpec | null {
  const key = dynamicColumnKey(row);
  if (!key || userHidden.value.has(key)) return null;
  const record = row as Record<string, unknown>;
  const rawType = record.columnType ?? record.type;
  const rawEditor = (row as Record<string, unknown>).editor;
  const editor =
    typeof rawEditor === "string" && rawEditor
      ? (rawEditor as ColumnSpec["editor"])
      : defaultColumnEditor(rawType);
  return {
    key,
    label: dynamicColumnLabel(row, key),
    type: defaultColumnType(rawType),
    editable: row.editable === true,
    editor: row.editable === true ? editor : undefined,
    readOnly: row.readOnly === true,
    nullable: row.nullable === true,
  };
}

// The column set belongs to the resource, not to the page: fetch it once per
// stateKey so paging/sorting/refreshing never blanks or re-requests it.
async function loadDynamicColumns(): Promise<void> {
  if (declaredColumns.value?.length || !columnsSource.value) return;
  if (columnsLoading.value || columnsLoaded.value) return;
  columnsLoading.value = true;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      columnsSource.value,
      { resource: props.resource, record: props.record },
      { limit: 500 },
    );
    dynamicColumns.value = page.items
      .map(dynamicColumn)
      .filter((col): col is ColumnSpec => Boolean(col));
    columnsLoaded.value = true;
  } finally {
    columnsLoading.value = false;
  }
}

function linkRef(row: Row, col: ColumnSpec): ResourceIdentity | null {
  const ref = row._links?.[col.key];
  return ref && row[col.key] != null && row[col.key] !== "" ? ref : null;
}
function openLink(ref: ResourceIdentity): void {
  emit("select", { ref } as Row);
}

const jsonEdit = ref<{
  row: Row;
  col: ColumnSpec;
  text: string;
  error: string | null;
  saving: boolean;
} | null>(null);

function openJsonEdit(row: Row, col: ColumnSpec): void {
  jsonEdit.value = {
    row,
    col,
    text: jsonEditorText(row[col.key]),
    error: null,
    saving: false,
  };
}

function closeJsonEdit(): void {
  if (jsonEdit.value?.saving) return;
  jsonEdit.value = null;
}

function updateJsonEditText(value: string): void {
  if (jsonEdit.value) jsonEdit.value.text = value;
}

async function saveJsonEdit(): Promise<void> {
  const edit = jsonEdit.value;
  if (!edit) return;
  let value: unknown;
  const raw = edit.text.trim();
  if (!raw && edit.col.nullable) value = null;
  else {
    try {
      value = JSON.parse(raw);
    } catch (err) {
      edit.error = (err as Error).message;
      return;
    }
  }
  const prev = edit.row[edit.col.key];
  edit.saving = true;
  edit.error = null;
  const ok = await commitCellValue(edit.row, edit.col, prev, value);
  edit.saving = false;
  if (ok) jsonEdit.value = null;
}

function formatNumber(v: number, col: ColumnSpec): string {
  const n = col.precision != null ? v.toFixed(col.precision) : String(v);
  return col.type === ColumnType.Percent ? `${n}%` : n;
}

const relativeNow = ref(Date.now());
const hasRelativeTimeColumn = computed(() =>
  columns.value.some((col) => col.type === ColumnType.RelativeTime),
);

function formatRelativeTime(v: string): string {
  const ts = Date.parse(v);
  if (Number.isNaN(ts)) return v;
  const seconds = Math.floor(Math.max(0, relativeNow.value - ts) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function display(row: Row, col: ColumnSpec): string {
  const v = row[col.key];
  if (v === undefined || v === null || v === "") return "—";
  if (col.type === ColumnType.Icon) {
    if (typeof v === "string") return v;
    if (typeof v === "object" && "value" in v) return String(v.value);
    return "—";
  }
  if (col.type === ColumnType.Bytes && typeof v === "number")
    return formatBytes(v);
  if (col.type === ColumnType.Duration && typeof v === "number")
    return formatDurationSeconds(v);
  if (
    (col.type === ColumnType.Number || col.type === ColumnType.Percent) &&
    typeof v === "number"
  )
    return formatNumber(v, col);
  if (col.type === ColumnType.RelativeTime && typeof v === "string")
    return formatRelativeTime(v);
  if (col.type === ColumnType.DateTime && typeof v === "string") {
    const at = new Date(v);
    return Number.isNaN(at.getTime()) ? v : DATE_FMT.format(at);
  }
  if (isStructuredValue(v)) return structuredSummary(v);
  return String(v);
}

function displayTitle(row: Row, col: ColumnSpec): string {
  const v = row[col.key];
  if (v === undefined || v === null || v === "") return "—";
  return isStructuredValue(v) ? fullCellText(v) : display(row, col);
}

function badgeClass(row: Row, col: ColumnSpec): string {
  return badgeClassFor(col.severities, row[col.key]);
}

function iconCell(row: Row, col: ColumnSpec): Icon | null {
  const v = row[col.key];
  if (!v) return null;
  if (typeof v === "string") return { type: "lucide", value: v };
  if (
    typeof v === "object" &&
    "type" in v &&
    "value" in v &&
    typeof v.type === "string" &&
    typeof v.value === "string"
  ) {
    return v as Icon;
  }
  return null;
}

function columnWidth(col: ColumnSpec): string {
  return (
    col.width ||
    TYPE_COLUMN_WIDTH[col.type ?? ColumnType.Text] ||
    DEFAULT_COLUMN_WIDTH
  );
}

function columnStyle(col: ColumnSpec): Record<string, string> {
  const width = columnWidth(col);
  const fixedMinimum = Boolean(col.width) || col.type === ColumnType.Icon;
  // No maxWidth: it is undefined on table cells and ignored under table-layout:auto.
  // The cap lives on the cell/header content instead.
  return { minWidth: fixedMinimum ? width : "7.5rem", width };
}

function cellClass(row: Row, col: ColumnSpec): string {
  if (col.type === ColumnType.Icon) return "flex min-w-0 justify-center";
  const base = cn(
    "group/cell flex min-w-0 items-center gap-1 truncate",
    canEditCell(col) &&
      "rounded px-1.5 py-0.5 transition-colors hover:bg-primary-50 focus-within:bg-primary-50 dark:hover:bg-primary-500/10 dark:focus-within:bg-primary-500/10",
  );
  if (staged.value && isEdited(row, col.key)) {
    return cn(
      base,
      "bg-amber-100 font-medium text-amber-900 dark:bg-amber-500/20 dark:text-amber-100",
    );
  }
  return base;
}

function blockPendingRowReplacement(): boolean {
  if (pendingCount.value === 0) return false;
  toast.add({
    severity: "warn",
    summary: "Unsaved changes",
    detail: "Commit or discard table changes before replacing these rows.",
    life: 5000,
  });
  return true;
}

async function confirmRowReplacement(
  action: () => void | Promise<void>,
): Promise<boolean> {
  return confirmBeforeDiscard(async () => {
    discardStaged();
    await action();
  });
}

async function load(targetFirst = first.value): Promise<void> {
  if (!props.source) return;
  if (blockPendingRowReplacement()) return;
  loading.value = true;
  error.value = null;
  selection.value = [];
  clearStaging();
  const seq = ++loadSeq;
  try {
    await loadDynamicColumns();
    const page = await fetchPage<Row>(
      props.connectionId,
      props.source,
      { resource: props.resource, record: props.record },
      {
        cursor: cursorFor(targetFirst),
        limit: pageSize.value,
        filter: filterText.value ? { q: filterText.value } : undefined,
        sort: sortField.value
          ? [{ field: sortField.value, desc: sortOrder.value === -1 }]
          : undefined,
      },
    );
    if (seq !== loadSeq) return;
    page.items.forEach(assignRid);
    rememberNextCursor(targetFirst, page);
    rows.value = page.items;
    hasMore.value = Boolean(page.nextCursor);
    total.value = page.total;
    first.value = targetFirst;
  } catch (e) {
    if (seq === loadSeq) error.value = (e as Error).message;
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}

// An explicit refresh re-reads the schema; paging and sorting keep the cache.
async function guardedLoad(targetFirst = first.value): Promise<void> {
  await confirmRowReplacement(async () => {
    columnsLoaded.value = false;
    await load(targetFirst);
  });
}

function onSort(e: DataTableSortEvent): void {
  void confirmRowReplacement(async () => {
    sortField.value = (e.sortField as string) ?? undefined;
    sortOrder.value = e.sortOrder ?? undefined;
    first.value = 0;
    resetCursors();
    saveTableState();
    await load(0);
  });
}

function onPage(e: DataTablePageEvent): void {
  void confirmRowReplacement(async () => {
    if (e.rows !== pageSize.value) resetCursors();
    first.value = e.first;
    pageSize.value = e.rows;
    saveTableState();
    await load(e.first);
  });
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

const navigableKinds = useNavigableKinds();
const rowClickMode = computed(() => tableConfig.value?.rowClick);
const detailEnabled = computed(
  () => rowClickMode.value === RowClickAction.Detail,
);
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

function toggleInSelection(row: Row): void {
  const id = rid(row);
  selection.value = selection.value.some((r) => rid(r) === id)
    ? selection.value.filter((r) => rid(r) !== id)
    : [...selection.value, row];
}

function onRowClick(e: DataTableRowClickEvent): void {
  const row = e.data as Row;
  const target = e.originalEvent?.target ?? null;
  if (isInteractiveTarget(target)) return;
  if (
    selectable.value &&
    target instanceof Element &&
    target.closest('[data-p-selection-column="true"]')
  ) {
    toggleInSelection(row);
    return;
  }
  if (editableCells.value) return; // body reserved for cell editing
  activateRow(row);
}

function activateRow(row: Row): void {
  switch (rowClickMode.value) {
    case RowClickAction.None:
      return;
    case RowClickAction.Detail:
      detailRow.value = row;
      return;
    case RowClickAction.Select:
      toggleSelection(row);
      return;
    case RowClickAction.Navigate:
      if (row.ref) emit("select", row);
      return;
  }
  if (navigates(row) || (row.ref && !selectable.value)) emit("select", row);
  else if (selectable.value) toggleSelection(row);
}

// Keyboard parity for clickable rows: PrimeVue's DataTable only wires row
// keyboard nav when selectionMode is set, which this panel doesn't use.
function onRowKeydown(e: KeyboardEvent, row: Row): void {
  if (e.key !== "Enter" && e.key !== " ") return;
  if (isInteractiveTarget(e.target)) return;
  e.preventDefault();
  activateRow(row);
}

const rowPT = new WeakMap<Row, Record<string, unknown>>();
function bodyRowPT(options: {
  context?: { index?: number };
}): Record<string, unknown> {
  const row = rows.value[options?.context?.index ?? -1];
  if (!row || editableCells.value || !rowClickable(row)) return {};
  const cached = rowPT.get(row);
  if (cached) return cached;
  const pt = {
    tabindex: 0,
    onKeydown: (e: KeyboardEvent) => onRowKeydown(e, row),
  };
  rowPT.set(row, pt);
  return pt;
}

function rowClickable(row: Row): boolean {
  if (editableCells.value) return false;
  const mode = rowClickMode.value;
  if (mode) return mode !== RowClickAction.None;
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
      text: isStructuredValue(r[col.key])
        ? fullCellText(r[col.key])
        : display(r, col),
      badge: col.type === ColumnType.Badge ? badgeClass(r, col) : undefined,
    });
  }
  for (const key of Object.keys(r)) {
    if (declared.has(key) || hidden.value.has(key)) continue;
    const v = (r as Record<string, unknown>)[key];
    if (v === undefined || v === null || v === "") continue;
    items.push({
      key,
      label: humanize(key),
      text: isStructuredValue(v) ? fullCellText(v) : String(v),
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
  await guardedLoad(first.value);
  emit("actionDone", action, result);
}

function hasServerViewState(): boolean {
  return Boolean(
    filterText.value.trim() ||
    sortField.value ||
    first.value > 0 ||
    pageSize.value !== 50,
  );
}

let pendingEvents: ResourceEvent[] = [];
let flushHandle: number | undefined;
let watchRefreshHandle: ReturnType<typeof setTimeout> | undefined;

function scheduleWatchRefresh(): void {
  if (!active.value || watchRefreshHandle) return;
  watchRefreshHandle = setTimeout(() => {
    watchRefreshHandle = undefined;
    void refresh();
  }, 100);
}

const MAX_PENDING_EVENTS = 500;

function applyEvent(ev: ResourceEvent): void {
  if (pendingCount.value > 0) return; // don't clobber buffered staged edits
  // rAF is throttled while the document is occluded; fall back to a refetch
  // rather than letting the socket grow the buffer without bound.
  if (pendingEvents.length > MAX_PENDING_EVENTS) {
    pendingEvents = [];
    scheduleWatchRefresh();
    return;
  }
  pendingEvents.push(ev);
  if (flushHandle === undefined)
    flushHandle = requestAnimationFrame(flushEvents);
}

function flushEvents(): void {
  flushHandle = undefined;
  const batch = pendingEvents;
  pendingEvents = [];
  if (!batch.length || pendingCount.value > 0) return;
  const serverView = hasServerViewState();
  const index = new Map<string, number>();
  rows.value.forEach((r, i) => {
    if (r.ref?.uid) index.set(r.ref.uid, i);
  });
  const next = rows.value.slice();
  const additions = new Map<string, Row>();
  const removed = new Set<number>();
  let refetch = false;
  for (const ev of batch) {
    const uid = ev.ref.uid;
    if (!uid) continue;
    const idx = index.get(uid);
    const type = String(ev.type).toLowerCase();
    if (type === "deleted") {
      if (idx !== undefined) removed.add(idx);
      additions.delete(uid);
    } else if (idx !== undefined) {
      // A visible row changed: patch it in place — no reorder (so rows the user is
      // reading don't jump) and no refetch, even under server sort/filter/paging.
      removed.delete(idx);
      if (ev.resource) next[idx] = { ...next[idx], ...(ev.resource as Row) };
    } else if (additions.has(uid)) {
      if (ev.resource) {
        const merged = { ...additions.get(uid)!, ...(ev.resource as Row) };
        assignRid(merged);
        additions.set(uid, merged);
      }
    } else if (type === "added" && ev.resource) {
      // A brand-new row. In a plain view, append it so existing rows keep their
      // place; under a server-side view its page/sort position is unknown, so fall
      // back to a single debounced refetch rather than guessing.
      if (serverView) refetch = true;
      else {
        const added = { ...(ev.resource as Row), ref: ev.ref };
        assignRid(added);
        additions.set(uid, added);
      }
    }
    // A "modified" event for a row not on the current page is ignored — applying it
    // would invent a row that doesn't belong to this view.
  }
  const kept = removed.size ? next.filter((_, i) => !removed.has(i)) : next;
  rows.value = additions.size ? [...kept, ...additions.values()] : kept;
  if (total.value !== undefined && (additions.size || removed.size)) {
    total.value = Math.max(0, total.value + additions.size - removed.size);
  }
  if (refetch) scheduleWatchRefresh();
}

let stopWatch: (() => void) | undefined;
function stopResourceWatch(): void {
  stopWatch?.();
  stopWatch = undefined;
}

function startWatch(): void {
  stopResourceWatch();
  if (!active.value) return;
  const ds = refreshMs.value > 0 ? undefined : watchSource.value;
  stopWatch = ds
    ? watchResource(
        props.connectionId,
        ds,
        { resource: props.resource, record: props.record },
        applyEvent,
      )
    : undefined;
}

const refreshMs = computed(() => tableConfig.value?.refreshIntervalMs ?? 0);
const visibility = useDocumentVisibility();
const active = ref(true);
onActivated(() => {
  if (active.value) return;
  active.value = true;
  if (refreshMs.value === 0 && watchSource.value) {
    void refresh();
    startWatch();
  }
});
onDeactivated(() => {
  active.value = false;
  stopResourceWatch();
});

function canAutoRefresh(): boolean {
  if (!props.source || loading.value || committing.value) return false;
  if (refreshing.value) return false;
  // The edited cell can unmount without a complete/cancel event, so the flag is
  // only meaningful while its row is still on screen.
  if (editingRid.value && rows.value.some((r) => rid(r) === editingRid.value))
    return false;
  if (pendingCount.value > 0) return false;
  if (
    showInsert.value ||
    deleteTarget.value ||
    actionOutput.value ||
    jsonEdit.value
  )
    return false;
  if (detailRow.value) return false;
  return true;
}

useConnectionInvalidationRefresh({
  connectionId: () => props.connectionId,
  refresh,
  active,
  canRefresh: canAutoRefresh,
});

async function refresh(): Promise<void> {
  if (!canAutoRefresh()) return;
  const source = props.source;
  if (!source) return;
  const seq = ++loadSeq;
  refreshing.value = true;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      source,
      { resource: props.resource, record: props.record },
      {
        cursor: cursorFor(first.value),
        limit: pageSize.value,
        filter: filterText.value ? { q: filterText.value } : undefined,
        sort: sortField.value
          ? [{ field: sortField.value, desc: sortOrder.value === -1 }]
          : undefined,
      },
    );
    if (seq !== loadSeq) return;
    page.items.forEach(assignRid);
    rememberNextCursor(first.value, page);
    const keep = new Set(selection.value.map(rid));
    rows.value = page.items;
    selection.value = keep.size
      ? page.items.filter((r) => keep.has(rid(r)))
      : [];
    hasMore.value = Boolean(page.nextCursor);
    total.value = page.total;
    error.value = null;
  } catch {
    return;
  } finally {
    refreshing.value = false;
  }
}

const { pause: pausePoll, resume: resumePoll } = useIntervalFn(
  refresh,
  () => refreshMs.value || 1000,
  { immediate: false },
);

const { pause: pauseRelativeTime, resume: resumeRelativeTime } = useIntervalFn(
  () => {
    relativeNow.value = Date.now();
  },
  1000,
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

vueWatch(
  () =>
    hasRelativeTimeColumn.value &&
    active.value &&
    visibility.value === "visible",
  (on) => {
    if (!on) {
      pauseRelativeTime();
      return;
    }
    relativeNow.value = Date.now();
    resumeRelativeTime();
  },
  { immediate: true },
);

vueWatch(
  () => stateKey.value,
  () => {
    resetCursors();
    restoreTableState();
    rows.value = [];
    total.value = undefined;
    hasMore.value = false;
    error.value = null;
    selection.value = [];
    dynamicColumns.value = [];
    columnsLoaded.value = false;
    load(first.value);
    startWatch();
  },
  { immediate: true },
);

// Pause the live watch while the tab is hidden; on return, re-list to catch up on
// anything missed and resubscribe.
vueWatch(
  () => visibility.value === "visible",
  (visible) => {
    if (!active.value || refreshMs.value > 0 || !watchSource.value) return;
    if (visible) {
      load(first.value);
      startWatch();
    } else {
      stopResourceWatch();
    }
  },
);

vueWatch([filterText, sortField, sortOrder, first, pageSize], () =>
  saveTableState(),
);

let debounce: ReturnType<typeof setTimeout> | undefined;
function onFilter(): void {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(() => {
    void confirmRowReplacement(async () => {
      first.value = 0;
      resetCursors();
      saveTableState();
      await load(0);
    });
  }, 250);
}

onUnmounted(() => {
  stopResourceWatch();
  pendingEvents = [];
  if (debounce) clearTimeout(debounce);
  if (watchRefreshHandle) clearTimeout(watchRefreshHandle);
  if (flushHandle !== undefined) cancelAnimationFrame(flushHandle);
});
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="@container flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-surface-200 px-4 py-2 dark:border-surface-800"
    >
      <div class="min-w-0 flex-1 basis-44 sm:w-56 sm:flex-none">
        <InputText
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
        size="small"
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
      <ActionBar
        v-if="globalActions.length"
        :connection-id="connectionId"
        :actions="globalActions"
        :resource="resource"
        :scope="source?.params"
        @done="onActionDone"
      />
      <div class="ml-auto flex min-w-0 shrink items-center gap-2">
        <Button
          v-if="canExport"
          type="button"
          size="small"
          severity="secondary"
          :disabled="!rows.length"
          title="Export loaded rows"
          aria-haspopup="true"
          @click="exportMenu?.toggle($event)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="14" />
          <span class="@max-md:hidden">Export</span>
        </Button>
        <Menu v-if="canExport" ref="exportMenu" :model="exportItems" popup />
        <Button
          type="button"
          size="small"
          :disabled="loading"
          severity="secondary"
          title="Refresh"
          @click="guardedLoad(first)"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'refresh-cw' }"
            :size="14"
            :loading="loading"
          />
          <span class="@max-md:hidden">Refresh</span>
        </Button>
      </div>
    </div>

    <div
      v-if="rowActions.length && selection.length"
      class="flex flex-wrap items-center gap-2 border-b border-surface-200 px-4 py-2 dark:border-surface-800"
    >
      <span class="text-xs text-surface-400"
        >{{ selection.length }} selected</span
      >
      <ActionBar
        :connection-id="connectionId"
        :actions="rowActions"
        :resource="selection.length === 1 ? (selection[0]?.ref ?? null) : null"
        :record="selection.length === 1 ? selection[0] : null"
        :records="selection"
        :max-inline="2"
        @done="onActionDone"
      />
    </div>

    <div
      v-if="staged && pendingCount"
      class="flex flex-wrap items-center gap-2 gap-y-1 border-b border-amber-300 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'git-commit-horizontal' }"
        :size="14"
      />
      <span class="min-w-0"
        >{{ pendingCount }} unsaved
        {{ pendingCount === 1 ? "change" : "changes" }}</span
      >
      <div class="ml-auto flex shrink-0 gap-2">
        <Button
          type="button"
          size="small"
          label="Discard"
          severity="secondary"
          :disabled="committing"
          @click="discardStaged"
        />
        <Button
          type="button"
          size="small"
          label="Commit"
          :loading="committing"
          :disabled="committing"
          @click="commitStaged"
        />
      </div>
    </div>

    <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PanelError
        v-if="error && !rows.length"
        :message="error"
        retryable
        @retry="guardedLoad(first)"
      />
      <SkeletonList v-else-if="loading && !rows.length" :rows="8" />
      <PanelError
        v-else-if="error"
        class="border-b border-surface-200 dark:border-surface-800"
        :message="error"
        retryable
        @retry="guardedLoad(first)"
      />
      <DataTable
        v-if="rows.length || (!loading && !error)"
        v-model:selection="selection"
        :value="rows"
        data-key="__rid"
        :edit-mode="editableCells ? 'cell' : undefined"
        lazy
        paginator
        :first="first"
        :rows="pageSize"
        :total-records="total ?? first + rows.length + (hasMore ? pageSize : 0)"
        :rows-per-page-options="[25, 50, 100, 250]"
        :paginator-template="
          total == null
            ? 'RowsPerPageDropdown PrevPageLink CurrentPageReport NextPageLink'
            : 'RowsPerPageDropdown FirstPageLink PrevPageLink CurrentPageReport NextPageLink LastPageLink'
        "
        current-page-report-template="{first} to {last} of {totalRecords}"
        removable-sort
        :sort-field="sortField"
        :sort-order="sortOrder"
        scrollable
        scroll-height="flex"
        :row-class="rowClass"
        :pt="{ bodyRow: bodyRowPT }"
        :pt-options="{ mergeProps: true }"
        @sort="onSort"
        @page="onPage"
        @row-click="onRowClick"
        @cell-edit-init="onCellEditInit"
        @cell-edit-cancel="editingRid = null"
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
          :sortable="col.sortable"
          :style="columnStyle(col)"
          :header-style="columnStyle(col)"
          :body-style="columnStyle(col)"
        >
          <template #header>
            <span
              class="block min-w-0 truncate"
              :style="{ maxWidth: columnWidth(col) }"
              :title="col.label"
              >{{ col.label }}</span
            >
          </template>
          <template #body="{ data }">
            <span
              data-test="table-cell-value"
              :class="cellClass(data as Row, col)"
              :style="{ maxWidth: columnWidth(col) }"
              :title="displayTitle(data as Row, col)"
            >
              <Button
                v-if="linkRef(data as Row, col)"
                type="button"
                size="small"
                text
                severity="secondary"
                :pt="{
                  root: 'inline-flex min-w-0 max-w-full items-center gap-1 p-0 text-primary-600 hover:underline dark:text-primary-400',
                }"
                @click.stop="openLink(linkRef(data as Row, col)!)"
              >
                <span class="truncate">{{ display(data as Row, col) }}</span>
                <AppIcon
                  :icon="{ type: 'lucide', value: 'arrow-up-right' }"
                  :size="12"
                />
              </Button>
              <span
                v-else-if="col.type === 'badge'"
                class="max-w-full min-w-0 truncate rounded-full px-2 py-0.5 align-bottom text-xs"
                :class="badgeClass(data as Row, col)"
                >{{ display(data as Row, col) }}</span
              >
              <AppIcon
                v-else-if="col.type === 'icon' && iconCell(data as Row, col)"
                :icon="iconCell(data as Row, col)"
                :size="16"
              />
              <span v-else class="min-w-0 truncate">{{
                display(data as Row, col)
              }}</span>
              <Button
                v-if="canJsonEditCell(col) && !linkRef(data as Row, col)"
                type="button"
                size="small"
                text
                rounded
                severity="secondary"
                title="Edit JSON"
                aria-label="Edit JSON"
                :pt="{
                  root: 'ml-auto shrink-0 p-0.5 opacity-0 transition-opacity group-hover/cell:opacity-100 focus:opacity-100',
                }"
                @click.stop="openJsonEdit(data as Row, col)"
              >
                <AppIcon
                  :icon="{ type: 'lucide', value: 'pencil' }"
                  :size="13"
                />
              </Button>
              <AppIcon
                v-else-if="canInlineEditCell(col) && !linkRef(data as Row, col)"
                class="ml-auto shrink-0 opacity-0 transition-opacity group-hover/cell:opacity-70"
                :icon="{ type: 'lucide', value: 'pencil' }"
                :size="13"
              />
            </span>
          </template>
          <template v-if="canInlineEditCell(col)" #editor="{ data, field }">
            <Select
              v-if="col.editor === ColumnEditor.Select"
              v-model="data[field]"
              :options="col.options ?? []"
              option-label="label"
              option-value="value"
              class="w-full"
            />
            <div
              v-else-if="col.editor === ColumnEditor.Toggle"
              class="flex w-full items-center"
            >
              <ToggleSwitch v-model="data[field]" />
            </div>
            <InputNumber
              v-else-if="col.editor === ColumnEditor.Number"
              v-model="data[field]"
              :use-grouping="false"
              class="w-full"
              autofocus
            />
            <Textarea
              v-else-if="col.editor === ColumnEditor.Textarea"
              v-model="data[field]"
              rows="3"
              class="w-full"
              autofocus
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
              size="small"
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
          v-if="detailEnabled && !editableCells"
          :header-style="{ width: '3rem' }"
          :pt="{ bodyCell: 'w-12 text-right' }"
        >
          <template #body="{ data }">
            <Button
              type="button"
              size="small"
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
          size="small"
          label="Cancel"
          severity="secondary"
          @click="showInsert = false"
        />
        <Button
          type="button"
          size="small"
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
          size="small"
          label="Cancel"
          severity="secondary"
          :disabled="deleteBusy"
          @click="closeDeleteDialog"
        />
        <Button
          type="button"
          size="small"
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
        class="max-h-[60vh] overflow-auto rounded-lg bg-surface-50 p-4 text-xs leading-relaxed text-surface-800 dark:bg-surface-950 dark:text-surface-100"
        >{{ actionOutput?.output || "(no output)" }}</pre
      >
      <p v-if="actionOutput?.truncated" class="mt-2 text-xs text-amber-500">
        Output truncated.
      </p>
      <template #footer>
        <Button
          type="button"
          size="small"
          label="Close"
          severity="secondary"
          @click="actionOutput = null"
        />
      </template>
    </Dialog>

    <JsonCellDialog
      :visible="!!jsonEdit"
      :title="jsonEdit ? `Edit ${jsonEdit.col.label}` : 'Edit JSON'"
      :text="jsonEdit?.text ?? ''"
      :error="jsonEdit?.error"
      :saving="jsonEdit?.saving"
      @update:visible="(v) => !v && closeJsonEdit()"
      @update:text="updateJsonEditText"
      @save="saveJsonEdit"
    />

    <RowDetailDialog
      :visible="!!detailRow"
      :title="detailTitle"
      :items="detailItems"
      @update:visible="(v) => !v && (detailRow = null)"
    />
  </div>
</template>
