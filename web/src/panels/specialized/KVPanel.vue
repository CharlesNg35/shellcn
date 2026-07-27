<script setup lang="ts">
import { computed, onUnmounted, ref, shallowRef, triggerRef, watch } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import SelectButton from "primevue/selectbutton";
import VirtualScroller from "primevue/virtualscroller";
import Badge from "primevue/badge";
import { useToast } from "primevue/usetoast";
import { fetchDoc, fetchPage, runFormAction } from "@/api/dataSource";
import type { KVPanelConfig, Page } from "@/types/projection";
import type { PanelProps } from "../core/types";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import AppIcon from "@/components/AppIcon.vue";
import { dialogRoot } from "@/primevue/preset";
import { useDirtyGuard } from "../shared/useDirtyGuard";
import { useConnectionInvalidationRefresh } from "../shared/useConnectionInvalidationRefresh";

interface KVEntry {
  key: string;
  type?: string;
  ttl?: number;
  size?: number;
  value?: unknown;
}

interface KVDetail extends KVEntry {
  encoding?: string;
}

interface RowSelectEvent {
  data: KVEntry;
}

interface KVLeaf {
  label: string;
  entry: KVEntry;
}

interface KVFolder {
  id: string;
  label: string;
  prefix: string;
  count: number;
  folders: KVFolder[];
  leaves: KVLeaf[];
}

interface KVRow {
  id: string;
  label: string;
  depth: number;
  folder?: KVFolder;
  entry?: KVEntry;
}

// A keyspace can be unbounded, so the panel never follows the scan cursor to
// exhaustion: one bounded fill lands on open and every further page needs a
// "Load more" click, up to a hard in-memory cap past which the filter is the
// only way on. Match semantics belong to the plugin, so the panel only forwards
// the term as `q`.
const PAGE_SIZE = 200;
const MAX_KEYS = 5000;
// A plugin page is a bounded number of scan round trips, not a bounded number of
// matches, so a selective filter over a large keyspace routinely comes back
// empty with a live cursor. Empty pages stay free: one gesture keeps pulling
// until it yields a key, for at most this many pages.
const FILL_ATTEMPTS = 10;
// Below this many rows the list renders in full (identical to a plain list and
// cheaper); above it PrimeVue's VirtualScroller keeps the mounted rows bounded.
const VIRTUAL_THRESHOLD = 200;
const ROW_HEIGHT = 32;
const TABLE_ROW_HEIGHT = 37;

const props = defineProps<PanelProps>();
const toast = useToast();

const entries = shallowRef<KVEntry[]>([]);
const selected = ref<KVEntry | null>(null);
const tableSelection = ref<KVEntry | null>(null);
const detail = ref<KVDetail | null>(null);
const editor = ref("");
const type = ref("string");
const filterText = ref("");
const loading = ref(false);
const loadingMore = ref(false);
const loadingDetail = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const createOpen = ref(false);
const createKeyName = ref("");
const createType = ref("string");
const createValue = ref("");
const view = ref<"list" | "tree">("list");
const expandedKeys = ref<Record<string, boolean>>({});
const cursor = ref<string | undefined>(undefined);
const seenKeys = new Set<string>();
let detailRequest = 0;
let loadGen = 0;
let filterLoadHandle: ReturnType<typeof setTimeout> | undefined;
let filterPending = false;
const config = computed(() => props.config as KVPanelConfig | undefined);
const keyParam = computed(() => config.value?.keyParam ?? "key");
const writable = computed(() => config.value?.writable === true);
const delimiter = computed(() => config.value?.delimiter ?? "");
const hasTree = computed(() => delimiter.value !== "");
const hasMore = computed(() => cursor.value !== undefined);
const capped = computed(() => entries.value.length >= MAX_KEYS);
const canLoadMore = computed(() => hasMore.value && !capped.value);
const filterActive = computed(() => filterText.value.trim() !== "");
const countLabel = computed(() => {
  const count = entries.value.length;
  const noun = count === 1 ? "key" : "keys";
  if (capped.value && hasMore.value) return `${count}+ ${noun}`;
  return hasMore.value
    ? `Showing ${count} ${noun} (+more)`
    : `${count} ${noun}`;
});
const viewOptions = [
  { value: "list", icon: { type: "lucide" as const, value: "list" } },
  { value: "tree", icon: { type: "lucide" as const, value: "folder-tree" } },
];
const typeOptions = computed(() =>
  (config.value?.valueTypes ?? []).map((value) => ({ label: value, value })),
);
const hasTypes = computed(() => typeOptions.value.length > 0);
const editorLanguage = computed(() =>
  type.value === "json" ||
  editor.value.trim().startsWith("{") ||
  editor.value.trim().startsWith("[")
    ? "json"
    : "plaintext",
);
const dirty = computed(() => {
  if (!writable.value || !detail.value) {
    return false;
  }
  const currentType = detail.value.type ?? selected.value?.type ?? "string";
  return (
    editor.value !== stringify(detail.value.value) || type.value !== currentType
  );
});
const { confirmBeforeDiscard } = useDirtyGuard({
  isDirty: () => dirty.value,
  header: "Discard unsaved key changes?",
  message: "This key has unsaved changes. Discard them and continue?",
});

function newFolder(prefix: string, label: string): KVFolder {
  return {
    id: `folder:${prefix}`,
    label,
    prefix,
    count: 0,
    folders: [],
    leaves: [],
  };
}

const treeRoot = shallowRef(newFolder("", ""));

function sortedIndex(list: { label: string }[], label: string): number {
  let low = 0;
  let high = list.length;
  while (low < high) {
    const mid = (low + high) >> 1;
    if (list[mid].label.localeCompare(label) < 0) low = mid + 1;
    else high = mid;
  }
  return low;
}

function childFolder(
  parent: KVFolder,
  label: string,
  prefix: string,
): KVFolder {
  const at = sortedIndex(parent.folders, label);
  const existing = parent.folders[at];
  if (existing?.label === label) return existing;
  const folder = newFolder(prefix, label);
  parent.folders.splice(at, 0, folder);
  return folder;
}

// insertKey splices one entry into the namespace tree in place so an appended
// page costs O(depth · log n) instead of rebuilding the tree from every key. A
// key that is also a prefix of others ("a" alongside "a:b") stays a leaf beside
// the folder; folder and key rows are keyed distinctly so they never collide.
// Folder counts are the number of leaf keys anywhere beneath them.
function insertKey(entry: KVEntry): void {
  const delim = delimiter.value;
  let segments = entry.key.split(delim);
  // A leading delimiter (e.g. "/a/b") is the root, not an empty folder.
  if (segments.length > 1 && segments[0] === "") segments = segments.slice(1);
  let folder = treeRoot.value;
  let prefix = "";
  for (let i = 0; i < segments.length - 1; i++) {
    prefix = i === 0 ? segments[i] : `${prefix}${delim}${segments[i]}`;
    folder = childFolder(folder, segments[i], prefix);
    folder.count++;
  }
  const label = segments[segments.length - 1];
  folder.leaves.splice(sortedIndex(folder.leaves, label), 0, { label, entry });
}

// A filter reveals its matches by expanding every folder by default, but an
// explicit toggle still wins so branches stay collapsible while filtering.
function isExpanded(row: KVRow): boolean {
  return expandedKeys.value[row.id] ?? filterActive.value;
}

function isSelected(row: KVRow): boolean {
  return row.entry !== undefined && row.entry.key === selected.value?.key;
}

// Only expanded branches are flattened, so a collapsed keyspace stays a handful
// of rows no matter how many keys are in memory.
const treeRows = computed<KVRow[]>(() => {
  const rows: KVRow[] = [];
  const walk = (folder: KVFolder, depth: number): void => {
    for (const child of folder.folders) {
      const row: KVRow = {
        id: child.id,
        label: child.label,
        depth,
        folder: child,
      };
      rows.push(row);
      if (isExpanded(row)) walk(child, depth + 1);
    }
    for (const leaf of folder.leaves) {
      rows.push({
        id: `key:${leaf.entry.key}`,
        label: leaf.label,
        depth,
        entry: leaf.entry,
      });
    }
  };
  walk(treeRoot.value, 0);
  return rows;
});
const treeVirtualized = computed(
  () => treeRows.value.length > VIRTUAL_THRESHOLD,
);
const tableVirtualScroller = computed(() =>
  entries.value.length > VIRTUAL_THRESHOLD
    ? { itemSize: TABLE_ROW_HEIGHT }
    : undefined,
);

function normalizeList(
  value: Page<KVEntry> | KVEntry[] | { items?: KVEntry[] },
) {
  if (Array.isArray(value)) return value;
  return value.items ?? [];
}

function stringify(value: unknown): string {
  return typeof value === "string"
    ? value
    : JSON.stringify(value ?? "", null, 2);
}

function activateEntry(entry: KVEntry | null): void {
  selected.value = entry;
  tableSelection.value = entry;
}

function resetKeys(): void {
  entries.value = [];
  seenKeys.clear();
  cursor.value = undefined;
  treeRoot.value = newFolder("", "");
}

function fetchKeyPage(from: string | undefined): Promise<Page<KVEntry>> {
  const search = filterText.value.trim();
  return fetchPage<KVEntry>(
    props.connectionId,
    props.source!,
    { resource: props.resource, record: props.record },
    {
      filter: search ? { q: search } : undefined,
      cursor: from,
      limit: PAGE_SIZE,
    },
  );
}

function appendPage(page: Page<KVEntry>): number {
  const added: KVEntry[] = [];
  for (const entry of normalizeList(page)) {
    if (seenKeys.size >= MAX_KEYS) break;
    if (seenKeys.has(entry.key)) continue;
    seenKeys.add(entry.key);
    added.push(entry);
  }
  cursor.value = page.nextCursor || undefined;
  if (!added.length) return 0;
  entries.value = entries.value.concat(added);
  if (hasTree.value) {
    for (const entry of added) insertKey(entry);
    triggerRef(treeRoot);
  }
  return added.length;
}

// Keeps following the cursor while the current gesture has produced nothing at
// all, so a key that only appears deep in the keyspace is still reachable
// without the user clicking "Load more" through a run of empty pages.
async function fillEmptyPages(gen: number): Promise<void> {
  for (let attempt = 0; attempt < FILL_ATTEMPTS; attempt++) {
    if (!canLoadMore.value || gen !== loadGen) return;
    const page = await fetchKeyPage(cursor.value);
    if (gen !== loadGen) return;
    if (appendPage(page) > 0) return;
  }
}

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  const gen = ++loadGen;
  filterPending = false;
  loading.value = true;
  error.value = null;
  const selectedKey = selected.value?.key;
  try {
    const page = await fetchKeyPage(undefined);
    if (gen !== loadGen) return;
    resetKeys();
    if (appendPage(page) === 0) await fillEmptyPages(gen);
    const next =
      entries.value.find((entry) => entry.key === selectedKey) ??
      entries.value[0] ??
      null;
    activateEntry(next);
    if (selected.value) await loadDetail(selected.value);
  } catch (e) {
    if (gen !== loadGen) return;
    error.value = (e as Error).message;
    // The rows on screen belong to the scan this reload was replacing, so the
    // cursor no longer has an owner: drop it rather than let "Load more" resume
    // an abandoned scan into a list it does not match.
    cursor.value = undefined;
  } finally {
    if (gen === loadGen) loading.value = false;
  }
}

// One bounded fill per invocation: the cursor is never followed to exhaustion,
// and the in-memory cap stops paging even if the user keeps clicking.
// A reload in flight blocks paging outright — its cursor belongs to the scan that
// is about to be discarded, so appending its page would splice keys from the old
// scan into the new one and leave the cursor pointing down the abandoned chain.
async function loadMore(): Promise<void> {
  if (!canLoadMore.value || loadingMore.value || loading.value || !props.source)
    return;
  const gen = loadGen;
  loadingMore.value = true;
  try {
    const page = await fetchKeyPage(cursor.value);
    if (gen !== loadGen) return;
    if (appendPage(page) === 0) await fillEmptyPages(gen);
  } catch (e) {
    if (gen !== loadGen) return;
    toast.add({
      severity: "error",
      summary: "Could not load more keys",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    loadingMore.value = false;
  }
}

useConnectionInvalidationRefresh({
  connectionId: () => props.connectionId,
  refresh: load,
  canRefresh: () => !dirty.value && !saving.value && !createOpen.value,
});

async function loadDetail(entry: KVEntry): Promise<void> {
  const request = ++detailRequest;
  activateEntry(entry);
  detail.value = null;
  editor.value = "";
  type.value = entry.type ?? "string";
  const routeId = config.value?.readRouteId;
  if (!routeId) {
    detail.value = entry;
    editor.value = stringify(entry.value);
    type.value = entry.type ?? "string";
    return;
  }
  loadingDetail.value = true;
  try {
    const loaded = await fetchDoc<KVDetail>(
      props.connectionId,
      { routeId, params: { [keyParam.value]: entry.key } },
      { resource: props.resource, record: props.record },
    );
    if (request !== detailRequest) return;
    detail.value = loaded;
    editor.value = stringify(detail.value.value);
    type.value = detail.value.type ?? entry.type ?? "string";
  } catch (e) {
    if (request !== detailRequest) return;
    detail.value = null;
    editor.value = "";
    toast.add({
      severity: "error",
      summary: "Could not load key",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    if (request === detailRequest) loadingDetail.value = false;
  }
}

async function guardedLoad(): Promise<void> {
  await confirmBeforeDiscard(load);
}

// A new filter is a new scan: paging restarts from the server with the pattern
// pushed down, instead of narrowing what happens to be in memory. Unsaved edits
// hold the reload back rather than cancel it, so the filter is honoured as soon
// as the editor is clean again.
function queueFilterLoad(): void {
  if (filterLoadHandle) clearTimeout(filterLoadHandle);
  filterLoadHandle = setTimeout(() => {
    filterLoadHandle = undefined;
    filterPending = dirty.value;
    if (!filterPending) void load();
  }, 250);
}

async function guardedLoadDetail(entry: KVEntry): Promise<boolean> {
  return confirmBeforeDiscard(() => loadDetail(entry));
}

async function selectRow(event: RowSelectEvent): Promise<void> {
  if (event.data.key === selected.value?.key) {
    tableSelection.value = selected.value;
    return;
  }
  const selectedChanged = await guardedLoadDetail(event.data);
  if (!selectedChanged) {
    tableSelection.value = selected.value;
  }
}

async function activateRow(row: KVRow): Promise<void> {
  if (row.entry) {
    await guardedLoadDetail(row.entry);
    return;
  }
  expandedKeys.value = {
    ...expandedKeys.value,
    [row.id]: !isExpanded(row),
  };
}

function restoreSelection(): void {
  tableSelection.value = selected.value;
}

async function save(): Promise<void> {
  if (!selected.value || !detail.value || !config.value?.writeRouteId) return;
  saving.value = true;
  try {
    await runFormAction(
      props.connectionId,
      config.value.writeRouteId,
      { resource: props.resource, record: props.record },
      { key: selected.value.key, type: type.value, value: editor.value },
      { [keyParam.value]: selected.value.key },
      "PUT",
    );
    await load();
  } catch (e) {
    toast.add({
      severity: "error",
      summary: "Save failed",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    saving.value = false;
  }
}

async function createKey(): Promise<void> {
  if (!config.value?.createRouteId) return;
  const key = createKeyName.value.trim();
  if (!key) return;
  saving.value = true;
  try {
    await runFormAction(
      props.connectionId,
      config.value.createRouteId,
      { resource: props.resource, record: props.record },
      { key, type: createType.value, value: createValue.value },
      { [keyParam.value]: key },
      "PUT",
    );
    toast.add({ severity: "success", summary: "Key created", life: 2200 });
    createOpen.value = false;
    createKeyName.value = "";
    createValue.value = "";
    await load();
    const created = entries.value.find((entry) => entry.key === key);
    if (created) await loadDetail(created);
  } catch (e) {
    toast.add({
      severity: "error",
      summary: "Create failed",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    saving.value = false;
  }
}

async function remove(): Promise<void> {
  if (!selected.value || !config.value?.deleteRouteId) return;
  saving.value = true;
  try {
    await runFormAction(
      props.connectionId,
      config.value.deleteRouteId,
      { resource: props.resource, record: props.record },
      {},
      { [keyParam.value]: selected.value.key },
      "DELETE",
    );
    toast.add({ severity: "success", summary: "Key deleted", life: 2200 });
    detail.value = null;
    activateEntry(null);
    await load();
  } catch (e) {
    toast.add({
      severity: "error",
      summary: "Delete failed",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    saving.value = false;
  }
}

watch(hasTree, (has) => (view.value = has ? "tree" : "list"), {
  immediate: true,
});

// The tree is spliced together as pages land, so a delimiter that arrives (or
// changes) after the fact has to rebuild it from the keys already in memory.
watch(delimiter, () => {
  treeRoot.value = newFolder("", "");
  if (hasTree.value) {
    for (const entry of entries.value) insertKey(entry);
  }
  triggerRef(treeRoot);
});

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});

watch(filterText, queueFilterLoad);

// Toggles made under one filter state say nothing about the next one, so
// turning the filter on or off collapses back to the default.
watch(filterActive, () => (expandedKeys.value = {}));

// A filter reload skipped because of unsaved edits runs as soon as they resolve.
watch(dirty, (isDirty) => {
  if (isDirty || !filterPending) return;
  filterPending = false;
  void load();
});

onUnmounted(() => {
  if (filterLoadHandle) clearTimeout(filterLoadHandle);
});
</script>

<template>
  <div
    class="grid h-full min-h-0 w-full grid-cols-[minmax(12rem,min(22rem,40%))_minmax(0,1fr)] overflow-hidden"
  >
    <div
      class="flex min-h-0 min-w-0 flex-col border-r border-surface-200 dark:border-surface-800"
    >
      <div
        class="flex flex-wrap items-center gap-2 border-b border-surface-200 p-3 dark:border-surface-800"
      >
        <InputText
          v-model="filterText"
          placeholder="Filter keys…"
          aria-label="Filter keys"
          class="min-w-0 flex-1 basis-40"
        />
        <SelectButton
          v-if="hasTree"
          v-model="view"
          :options="viewOptions"
          option-value="value"
          :allow-empty="false"
          aria-label="Key view"
          class="shrink-0"
        >
          <template #option="{ option }">
            <AppIcon :icon="option.icon" :size="14" />
          </template>
        </SelectButton>
        <Button
          type="button"
          severity="secondary"
          class="shrink-0"
          :disabled="loading"
          @click="guardedLoad"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'refresh-cw' }"
            :size="14"
            :loading="loading"
          />
          Refresh
        </Button>
        <Button
          v-if="writable && config?.createRouteId"
          type="button"
          label="New"
          class="shrink-0"
          :disabled="saving"
          @click="createOpen = true"
        />
      </div>

      <PanelError
        v-if="error && !entries.length"
        :message="error"
        retryable
        @retry="guardedLoad"
      />
      <SkeletonList v-else-if="loading && !entries.length" :rows="8" />
      <template v-else>
        <PanelError
          v-if="error"
          class="border-b border-surface-200 dark:border-surface-800"
          :message="error"
          retryable
          @retry="guardedLoad"
        />
        <div
          v-if="!entries.length"
          class="min-h-0 flex-1 px-4 py-8 text-center text-sm text-surface-400"
        >
          <p>No keys.</p>
          <p v-if="!filterActive" class="mt-1 text-xs">
            Large keyspaces load one page at a time — filter to narrow the scan.
          </p>
        </div>
        <div
          v-else-if="view === 'tree'"
          class="min-h-0 flex-1 overflow-y-auto p-2 text-sm"
          data-test="kv-tree"
        >
          <VirtualScroller
            :items="treeRows"
            :item-size="ROW_HEIGHT"
            :disabled="!treeVirtualized"
            scroll-height="100%"
            class="h-full"
          >
            <template
              #content="{ items: rows, styleClass, contentRef, contentStyle }"
            >
              <div
                :ref="contentRef"
                :class="styleClass"
                :style="contentStyle"
                role="tree"
                aria-label="Keys"
              >
                <button
                  v-for="row in rows as KVRow[]"
                  :key="row.id"
                  type="button"
                  role="treeitem"
                  data-test="kv-row"
                  :aria-level="row.depth + 1"
                  :aria-expanded="row.folder ? isExpanded(row) : undefined"
                  :aria-selected="row.entry ? isSelected(row) : undefined"
                  :title="row.folder ? row.folder.prefix : row.entry?.key"
                  class="flex w-full cursor-pointer items-center gap-1.5 rounded-md pr-2 text-left transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
                  :class="
                    isSelected(row)
                      ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-200'
                      : 'text-surface-700 dark:text-surface-200'
                  "
                  :style="{
                    height: `${ROW_HEIGHT}px`,
                    paddingLeft: `${row.depth * 12 + 4}px`,
                  }"
                  @click="activateRow(row)"
                >
                  <AppIcon
                    :icon="{
                      type: 'lucide',
                      value: isExpanded(row) ? 'chevron-down' : 'chevron-right',
                    }"
                    :size="14"
                    class="shrink-0 text-surface-400"
                    :class="{ invisible: !row.folder }"
                  />
                  <AppIcon
                    v-if="row.folder"
                    :icon="{ type: 'lucide', value: 'folder' }"
                    :size="14"
                    class="shrink-0 text-amber-500"
                  />
                  <AppIcon
                    v-else
                    :icon="{ type: 'lucide', value: 'key-round' }"
                    :size="13"
                    class="shrink-0 text-surface-400"
                  />
                  <span class="min-w-0 flex-1 truncate">{{ row.label }}</span>
                  <span
                    v-if="row.folder"
                    class="shrink-0 text-xs text-surface-400 tabular-nums"
                    >{{ row.folder.count }}</span
                  >
                  <span
                    v-else-if="row.entry?.type"
                    class="shrink-0 rounded bg-surface-100 px-1 py-0.5 text-[10px] font-medium tracking-wide text-surface-500 uppercase dark:bg-surface-800 dark:text-surface-400"
                    >{{ row.entry.type }}</span
                  >
                </button>
              </div>
            </template>
          </VirtualScroller>
        </div>

        <DataTable
          v-else
          v-model:selection="tableSelection"
          :value="entries"
          data-key="key"
          scrollable
          scroll-height="flex"
          selection-mode="single"
          class="min-h-0 flex-1"
          :virtual-scroller-options="tableVirtualScroller"
          @row-select="selectRow"
          @row-unselect="restoreSelection"
        >
          <Column field="key" header="Key" />
          <Column field="type" header="Type" style="width: 6rem" />
          <template #empty>No keys.</template>
        </DataTable>

        <div
          v-if="entries.length || hasMore"
          class="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-surface-200 px-3 py-1.5 text-xs text-surface-400 dark:border-surface-800"
        >
          <span class="tabular-nums">{{ countLabel }}</span>
          <span v-if="capped && hasMore" class="min-w-0 truncate"
            >scanning paused — filter to narrow</span
          >
          <button
            v-else-if="canLoadMore"
            type="button"
            class="ml-auto rounded px-1.5 py-0.5 font-medium text-primary-600 transition-colors hover:bg-primary-50 disabled:opacity-60 dark:text-primary-300 dark:hover:bg-primary-500/10"
            :disabled="loadingMore || loading"
            @click="loadMore"
          >
            {{ loadingMore ? "Loading…" : "Load more" }}
          </button>
        </div>
      </template>
    </div>

    <div class="flex min-h-0 min-w-0 flex-col">
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-b border-surface-200 px-4 py-3 dark:border-surface-800"
      >
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium text-surface-900 dark:text-surface-0">
            {{ selected?.key ?? "No key selected" }}
          </p>
          <p v-if="detail" class="text-xs text-surface-400">
            {{ detail.type || "string" }}
            <span v-if="detail.ttl != null"> · TTL {{ detail.ttl }}</span>
          </p>
        </div>
        <div
          v-if="writable && selected"
          class="flex shrink-0 items-center gap-2"
        >
          <Badge
            v-if="dirty"
            value="Unsaved"
            severity="warn"
            aria-live="polite"
          />
          <Button
            v-if="config?.deleteRouteId"
            type="button"
            label="Delete"
            severity="danger"
            outlined
            :disabled="saving"
            @click="remove"
          />
          <Button
            v-if="config?.writeRouteId"
            type="button"
            label="Save"
            :loading="saving"
            :disabled="saving || loadingDetail || !detail || !dirty"
            @click="save"
          />
        </div>
      </div>

      <div v-if="!selected" class="p-6 text-sm text-surface-400">
        Select a key to inspect its value.
      </div>
      <div v-else class="flex min-h-0 flex-1 flex-col gap-3 p-4">
        <div v-if="hasTypes" class="w-40">
          <label class="mb-1 block text-xs text-surface-400">Type</label>
          <Select
            v-model="type"
            :options="typeOptions"
            option-label="label"
            option-value="value"
            :disabled="!writable"
            aria-label="Type"
          />
        </div>
        <CodeTextEditor
          v-model:value="editor"
          class="min-h-0 flex-1"
          :language="editorLanguage"
          :readonly="!writable"
          :disabled="loadingDetail"
          aria-label="Key value"
        />
      </div>
    </div>

    <Dialog
      v-model:visible="createOpen"
      modal
      header="Create key"
      :pt="{ root: dialogRoot('max-w-2xl') }"
    >
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs text-surface-400">Key</label>
          <InputText
            v-model="createKeyName"
            class="w-full"
            aria-label="New key"
            autofocus
          />
        </div>
        <div v-if="hasTypes" class="w-44">
          <label class="mb-1 block text-xs text-surface-400">Type</label>
          <Select
            v-model="createType"
            :options="typeOptions"
            option-label="label"
            option-value="value"
            aria-label="Type"
          />
        </div>
        <div class="h-56">
          <label class="mb-1 block text-xs text-surface-400">Value</label>
          <CodeTextEditor
            v-model:value="createValue"
            class="h-full"
            :language="
              createType === 'json' ||
              createValue.trim().startsWith('{') ||
              createValue.trim().startsWith('[')
                ? 'json'
                : 'plaintext'
            "
            aria-label="New key value"
          />
        </div>
      </div>
      <template #footer>
        <Button
          type="button"
          severity="secondary"
          outlined
          label="Cancel"
          @click="createOpen = false"
        />
        <Button
          type="button"
          label="Create"
          :loading="saving"
          :disabled="saving || !createKeyName.trim()"
          @click="createKey"
        />
      </template>
    </Dialog>
  </div>
</template>
