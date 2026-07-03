<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import SelectButton from "primevue/selectbutton";
import Tree from "primevue/tree";
import type { TreeNode as PVNode } from "primevue/treenode";
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

interface KVNodeData {
  kind: "folder" | "key";
  entry?: KVEntry;
  count?: number;
  prefix?: string;
}
type KVNode = PVNode & { data: KVNodeData; children?: KVNode[] };

// Keys are pulled in bounded cursor batches so a large key set never blocks the
// UI; each "Scan more" pulls up to this many additional keys.
const SCAN_BUDGET = 2000;

const props = defineProps<PanelProps>();
const toast = useToast();

const entries = ref<KVEntry[]>([]);
const selected = ref<KVEntry | null>(null);
const tableSelection = ref<KVEntry | null>(null);
const detail = ref<KVDetail | null>(null);
const editor = ref("");
const type = ref("string");
const filterText = ref("");
const loading = ref(false);
const scanning = ref(false);
const loadingDetail = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const createOpen = ref(false);
const createKeyName = ref("");
const createType = ref("string");
const createValue = ref("");
const view = ref<"list" | "tree">("list");
const expandedKeys = ref<Record<string, boolean>>({});
const scanCursor = ref<string | undefined>(undefined);
const seenKeys = new Set<string>();
let detailRequest = 0;
let filterLoadHandle: ReturnType<typeof setTimeout> | undefined;
const config = computed(() => props.config as KVPanelConfig | undefined);
const keyParam = computed(() => config.value?.keyParam ?? "key");
const writable = computed(() => config.value?.writable === true);
const delimiter = computed(() => config.value?.delimiter ?? "");
const hasTree = computed(() => delimiter.value !== "");
const hasMore = computed(() => scanCursor.value !== undefined);
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

const visibleEntries = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return entries.value;
  return entries.value.filter((entry) =>
    [entry.key, entry.type].some((value) =>
      String(value ?? "")
        .toLowerCase()
        .includes(q),
    ),
  );
});

const treeNodes = computed<KVNode[]>(() =>
  buildKeyTree(visibleEntries.value, delimiter.value),
);
const treeSelectionKeys = computed(() =>
  selected.value ? { [`key:${selected.value.key}`]: true } : {},
);

function leafNode(entry: KVEntry, label: string): KVNode {
  return { key: `key:${entry.key}`, label, leaf: true, data: { kind: "key", entry } };
}

// buildKeyTree groups keys into a namespace tree by splitting on the delimiter.
// A key that is also a prefix of others (e.g. "a" alongside "a:b") stays a leaf
// beside the folder; the two never collide because folder and key nodes are keyed
// distinctly. Folder counts are the number of leaf keys anywhere beneath them.
function buildKeyTree(list: KVEntry[], delim: string): KVNode[] {
  if (!delim) return list.map((entry) => leafNode(entry, entry.key));

  interface Folder {
    node: KVNode;
    folders: Map<string, Folder>;
    leaves: KVNode[];
  }
  const makeFolder = (prefix: string, label: string): Folder => ({
    node: {
      key: `folder:${prefix}`,
      label,
      leaf: false,
      children: [],
      data: { kind: "folder", count: 0, prefix },
    },
    folders: new Map(),
    leaves: [],
  });
  const rootFolders = new Map<string, Folder>();
  const rootLeaves: KVNode[] = [];

  for (const entry of list) {
    const segments = entry.key.split(delim);
    if (segments.length === 1) {
      rootLeaves.push(leafNode(entry, entry.key));
      continue;
    }
    let level = rootFolders;
    let prefix = "";
    const chain: Folder[] = [];
    for (let i = 0; i < segments.length - 1; i++) {
      prefix = i === 0 ? segments[i] : `${prefix}${delim}${segments[i]}`;
      let folder = level.get(segments[i]);
      if (!folder) {
        folder = makeFolder(prefix, segments[i]);
        level.set(segments[i], folder);
      }
      chain.push(folder);
      level = folder.folders;
    }
    chain[chain.length - 1].leaves.push(
      leafNode(entry, segments[segments.length - 1]),
    );
    for (const folder of chain) folder.node.data.count!++;
  }

  const byLabel = (a: KVNode, b: KVNode): number =>
    (a.label ?? "").localeCompare(b.label ?? "");
  const assemble = (folders: Map<string, Folder>, leaves: KVNode[]): KVNode[] => {
    const folderNodes = [...folders.values()]
      .map((folder) => {
        folder.node.children = assemble(folder.folders, folder.leaves);
        return folder.node;
      })
      .sort(byLabel);
    return [...folderNodes, ...[...leaves].sort(byLabel)];
  };
  return assemble(rootFolders, rootLeaves);
}

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

// The tree is built from a background scan that follows the page cursor: load()
// paints after the first page, then fills up to SCAN_BUDGET more keys without
// blocking the panel; "Scan more" resumes past the cap. A generation token lets a
// reload or a new filter cancel an in-flight background fill.
let scanGen = 0;

async function scanNextPage(): Promise<number> {
  if (!props.source) return 0;
  const search = filterText.value.trim();
  const page = await fetchPage<KVEntry>(
    props.connectionId,
    props.source,
    { resource: props.resource, record: props.record },
    { filter: search ? { q: search } : undefined, cursor: scanCursor.value },
  );
  let added = 0;
  for (const entry of normalizeList(page)) {
    if (seenKeys.has(entry.key)) continue;
    seenKeys.add(entry.key);
    entries.value.push(entry);
    added++;
  }
  scanCursor.value = page.nextCursor || undefined;
  return added;
}

async function fillToBudget(gen: number): Promise<void> {
  if (gen !== scanGen) return;
  scanning.value = true;
  try {
    let added = 0;
    while (scanCursor.value && added < SCAN_BUDGET && gen === scanGen) {
      added += await scanNextPage();
    }
  } catch (e) {
    if (gen === scanGen) {
      toast.add({
        severity: "error",
        summary: "Could not scan more keys",
        detail: (e as Error).message,
        life: 4000,
      });
    }
  } finally {
    if (gen === scanGen) scanning.value = false;
  }
}

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  const gen = ++scanGen;
  loading.value = true;
  error.value = null;
  const selectedKey = selected.value?.key;
  entries.value = [];
  seenKeys.clear();
  scanCursor.value = undefined;
  try {
    await scanNextPage();
    if (gen !== scanGen) return;
    const next =
      entries.value.find((entry) => entry.key === selectedKey) ??
      entries.value[0] ??
      null;
    activateEntry(next);
    if (selected.value) await loadDetail(selected.value);
  } catch (e) {
    error.value = (e as Error).message;
    loading.value = false;
    return;
  }
  loading.value = false;
  if (scanCursor.value) void fillToBudget(gen);
}

async function scanMore(): Promise<void> {
  await fillToBudget(scanGen);
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

function queueFilterLoad(): void {
  if (filterLoadHandle) clearTimeout(filterLoadHandle);
  filterLoadHandle = setTimeout(() => {
    filterLoadHandle = undefined;
    if (!dirty.value) void load();
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

async function onTreeSelect(node: PVNode): Promise<void> {
  const data = node.data as KVNodeData;
  if (data.kind === "key" && data.entry) {
    await guardedLoadDetail(data.entry);
    return;
  }
  expandedKeys.value = {
    ...expandedKeys.value,
    [String(node.key)]: !expandedKeys.value[String(node.key)],
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

// Reveal matches while filtering; collapse back to the roots once cleared.
watch([filterText, treeNodes], () => {
  if (view.value !== "tree") return;
  if (!filterText.value.trim()) {
    expandedKeys.value = {};
    return;
  }
  const keys: Record<string, boolean> = {};
  const walk = (nodes: KVNode[]): void => {
    for (const node of nodes) {
      if (node.leaf) continue;
      keys[String(node.key)] = true;
      if (node.children) walk(node.children);
    }
  };
  walk(treeNodes.value);
  expandedKeys.value = keys;
});

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});

watch(filterText, queueFilterLoad);

onUnmounted(() => {
  if (filterLoadHandle) clearTimeout(filterLoadHandle);
});
</script>

<template>
  <div class="grid h-full min-h-0 grid-cols-[22rem_minmax(0,1fr)]">
    <div
      class="flex min-h-0 flex-col border-r border-surface-200 dark:border-surface-800"
    >
      <div
        class="flex items-center gap-2 border-b border-surface-200 p-3 dark:border-surface-800"
      >
        <InputText
          v-model="filterText"
          placeholder="Filter keys"
          aria-label="Filter keys"
          class="min-w-0 flex-1"
        />
        <Button
          type="button"
          severity="secondary"
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
          :disabled="saving"
          @click="createOpen = true"
        />
      </div>

      <div
        v-if="hasTree || hasMore || scanning"
        class="flex items-center gap-2 border-b border-surface-200 px-3 py-1.5 dark:border-surface-800"
      >
        <SelectButton
          v-if="hasTree"
          v-model="view"
          :options="viewOptions"
          option-value="value"
          :allow-empty="false"
          aria-label="Key view"
        >
          <template #option="{ option }">
            <AppIcon :icon="option.icon" :size="14" />
          </template>
        </SelectButton>
        <div
          class="ml-auto flex items-center gap-2 text-xs text-surface-400 tabular-nums"
        >
          <span v-if="scanning" class="flex items-center gap-1">
            <AppIcon
              :icon="{ type: 'lucide', value: 'loader-circle' }"
              :size="12"
              class="animate-spin"
            />
            Scanning…
          </span>
          <span v-else>{{ entries.length }} keys{{ hasMore ? "+" : "" }}</span>
          <button
            v-if="hasMore && !scanning"
            type="button"
            class="rounded px-1.5 py-0.5 font-medium text-primary-600 transition-colors hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-primary-500/10"
            @click="scanMore"
          >
            Scan more
          </button>
        </div>
      </div>

      <PanelError
        v-if="error && !entries.length"
        :message="error"
        retryable
        @retry="guardedLoad"
      />
      <SkeletonList v-else-if="loading && !entries.length" :rows="8" />
      <PanelError
        v-else-if="error"
        class="border-b border-surface-200 dark:border-surface-800"
        :message="error"
        retryable
        @retry="guardedLoad"
      />
      <template v-else-if="entries.length || (!loading && !error)">
        <Tree
          v-if="view === 'tree'"
          v-model:expanded-keys="expandedKeys"
          :value="treeNodes"
          :selection-keys="treeSelectionKeys"
          selection-mode="single"
          class="min-h-0 flex-1"
          @node-select="onTreeSelect"
        >
          <template #default="{ node }">
            <span
              class="flex w-full items-center gap-1.5"
              :title="node.data.kind === 'key' ? node.data.entry?.key : node.data.prefix"
            >
              <template v-if="node.data.kind === 'folder'">
                <AppIcon
                  :icon="{ type: 'lucide', value: 'folder' }"
                  :size="14"
                  class="shrink-0 text-amber-500"
                />
                <span class="min-w-0 flex-1 truncate">{{ node.label }}</span>
                <span class="shrink-0 text-xs text-surface-400 tabular-nums">{{
                  node.data.count
                }}</span>
              </template>
              <template v-else>
                <AppIcon
                  :icon="{ type: 'lucide', value: 'key-round' }"
                  :size="13"
                  class="shrink-0 text-surface-400"
                />
                <span class="min-w-0 flex-1 truncate">{{ node.label }}</span>
                <span
                  v-if="node.data.entry?.type"
                  class="shrink-0 rounded bg-surface-100 px-1 py-0.5 text-[10px] font-medium tracking-wide text-surface-500 uppercase dark:bg-surface-800 dark:text-surface-400"
                  >{{ node.data.entry.type }}</span
                >
              </template>
            </span>
          </template>
          <template #empty>
            <p class="px-2 py-6 text-center text-sm text-surface-400">No keys.</p>
          </template>
        </Tree>

        <DataTable
          v-else
          v-model:selection="tableSelection"
          :value="visibleEntries"
          data-key="key"
          scrollable
          scroll-height="flex"
          selection-mode="single"
          @row-select="selectRow"
          @row-unselect="restoreSelection"
        >
          <Column field="key" header="Key" />
          <Column field="type" header="Type" style="width: 6rem" />
          <template #empty>No keys.</template>
        </DataTable>
      </template>
    </div>

    <div class="flex min-h-0 flex-col">
      <div
        class="flex items-center justify-between gap-3 border-b border-surface-200 px-4 py-3 dark:border-surface-800"
      >
        <div class="min-w-0">
          <p class="truncate font-medium text-surface-900 dark:text-surface-0">
            {{ selected?.key ?? "No key selected" }}
          </p>
          <p v-if="detail" class="text-xs text-surface-400">
            {{ detail.type || "string" }}
            <span v-if="detail.ttl != null"> · TTL {{ detail.ttl }}</span>
          </p>
        </div>
        <div v-if="writable && selected" class="flex items-center gap-2">
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
