<script setup lang="ts">
import { computed, ref, watch } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import { useToast } from "primevue/usetoast";
import { fetchDoc, fetchPage, runFormAction } from "../../api/dataSource";
import type { KVPanelConfig, Page } from "../../types/projection";
import type { PanelProps } from "../core/types";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "../../components/SkeletonList.vue";
import AppIcon from "../../components/AppIcon.vue";
import { dialogRoot } from "../../primevue/preset";

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

const props = defineProps<PanelProps>();
const toast = useToast();

const entries = ref<KVEntry[]>([]);
const selected = ref<KVEntry | null>(null);
const detail = ref<KVDetail | null>(null);
const editor = ref("");
const type = ref("string");
const filterText = ref("");
const loading = ref(false);
const loadingDetail = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const createOpen = ref(false);
const createKeyName = ref("");
const createType = ref("string");
const createValue = ref("");
const config = computed(() => props.config as KVPanelConfig | undefined);
const keyParam = computed(() => config.value?.keyParam ?? "key");
const writable = computed(() => config.value?.writable === true);
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

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const page = await fetchPage<KVEntry>(props.connectionId, props.source, {
      resource: props.resource,
    });
    entries.value = normalizeList(page);
    selected.value = entries.value[0] ?? null;
    if (selected.value) await loadDetail(selected.value);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function loadDetail(entry: KVEntry): Promise<void> {
  selected.value = entry;
  const routeId = config.value?.readRouteId;
  if (!routeId) {
    detail.value = entry;
    editor.value = stringify(entry.value);
    type.value = entry.type ?? "string";
    return;
  }
  loadingDetail.value = true;
  try {
    detail.value = await fetchDoc<KVDetail>(
      props.connectionId,
      { routeId, params: { [keyParam.value]: entry.key } },
      { resource: props.resource },
    );
    editor.value = stringify(detail.value.value);
    type.value = detail.value.type ?? entry.type ?? "string";
  } catch (e) {
    toast.add({
      severity: "error",
      summary: "Could not load key",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    loadingDetail.value = false;
  }
}

async function save(): Promise<void> {
  if (!selected.value || !config.value?.writeRouteId) return;
  saving.value = true;
  try {
    await runFormAction(
      props.connectionId,
      config.value.writeRouteId,
      { resource: props.resource },
      { key: selected.value.key, type: type.value, value: editor.value },
      { [keyParam.value]: selected.value.key },
      "PUT",
    );
    toast.add({ severity: "success", summary: "Key saved", life: 2200 });
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
      { resource: props.resource },
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
      { resource: props.resource },
      {},
      { [keyParam.value]: selected.value.key },
      "DELETE",
    );
    toast.add({ severity: "success", summary: "Key deleted", life: 2200 });
    detail.value = null;
    selected.value = null;
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

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
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
          @click="load"
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
      <PanelError
        v-if="error && !entries.length"
        :message="error"
        retryable
        @retry="load"
      />
      <SkeletonList v-else-if="loading && !entries.length" :rows="8" />
      <PanelError
        v-else-if="error"
        class="border-b border-surface-200 dark:border-surface-800"
        :message="error"
        retryable
        @retry="load"
      />
      <DataTable
        v-if="entries.length || (!loading && !error)"
        :value="visibleEntries"
        data-key="key"
        scrollable
        scroll-height="flex"
        selection-mode="single"
        @row-click="loadDetail($event.data as KVEntry)"
      >
        <Column field="key" header="Key" />
        <Column field="type" header="Type" style="width: 6rem" />
        <template #empty>No keys.</template>
      </DataTable>
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
            :disabled="saving"
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
