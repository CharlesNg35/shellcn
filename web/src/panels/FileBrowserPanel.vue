<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import FileUpload from "primevue/fileupload";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import InputText from "primevue/inputtext";
import { useToast } from "primevue/usetoast";
import {
  fetchDoc,
  fetchPage,
  routeURL,
  runAction,
  uploadFiles,
} from "../api/dataSource";
import type {
  FileBrowserConfig,
  FileContent,
  FileEntry,
} from "../types/projection";
import type { PanelProps } from "./types";
import AppIcon from "../components/AppIcon.vue";
import FilePreview from "./file/FilePreview.vue";
import { formatBytes } from "./file/fileTypes";

const props = defineProps<PanelProps>();
const toast = useToast();

const fileConfig = computed(
  () => props.config as FileBrowserConfig | undefined,
);
const pathParam = computed(() => fileConfig.value?.pathParam ?? "path");
const readRouteId = computed(() => fileConfig.value?.readRouteId);
const downloadRouteId = computed(() => fileConfig.value?.downloadRouteId);
const uploadRouteId = computed(() => fileConfig.value?.uploadRouteId);
const mkdirRouteId = computed(() => fileConfig.value?.mkdirRouteId);
const renameRouteId = computed(() => fileConfig.value?.renameRouteId);
const deleteRouteId = computed(() => fileConfig.value?.deleteRouteId);
const writable = computed(() => Boolean(fileConfig.value?.writable));
const multipleUpload = computed(() => fileConfig.value?.multipleUpload ?? true);
const uploadFieldName = computed(
  () => fileConfig.value?.uploadFieldName ?? "files",
);

const startPath = computed(
  () => props.source?.params?.[pathParam.value] ?? "/",
);
const cwd = ref(startPath.value);
const entries = ref<FileEntry[]>([]);
const loadingList = ref(false);
const listError = ref<string | null>(null);

const selected = ref<FileEntry | null>(null);
const content = ref<FileContent | null>(null);
const loadingContent = ref(false);
const mutating = ref(false);
const mkdirOpen = ref(false);
const renameOpen = ref(false);
const deleteOpen = ref(false);
const newFolderName = ref("");
const renameName = ref("");

const sorted = computed(() =>
  [...entries.value].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.localeCompare(b.name);
  }),
);

const crumbs = computed(() => {
  const parts = cwd.value.split("/").filter(Boolean);
  const acc: { label: string; path: string }[] = [{ label: "/", path: "/" }];
  let p = "";
  for (const part of parts) {
    p += `/${part}`;
    acc.push({ label: part, path: p });
  }
  return acc;
});

const operationCtx = computed(() => ({ resource: props.resource }));
const canUpload = computed(
  () => writable.value && Boolean(uploadRouteId.value),
);
const canMkdir = computed(() => writable.value && Boolean(mkdirRouteId.value));
const canRename = computed(
  () =>
    writable.value && Boolean(renameRouteId.value) && Boolean(selected.value),
);
const canDelete = computed(
  () =>
    writable.value && Boolean(deleteRouteId.value) && Boolean(selected.value),
);
const downloadHref = computed(() => {
  if (!downloadRouteId.value || !selected.value || selected.value.isDir)
    return "";
  return routeURL(
    props.connectionId,
    downloadRouteId.value,
    operationCtx.value,
    operationParams(selected.value.path),
  );
});

function operationParams(path: string): Record<string, string> {
  return { ...(props.source?.params ?? {}), [pathParam.value]: path };
}

async function loadList(path: string): Promise<void> {
  if (!props.source) return;
  loadingList.value = true;
  listError.value = null;
  selected.value = null;
  content.value = null;
  try {
    const page = await fetchPage<FileEntry>(props.connectionId, {
      routeId: props.source.routeId,
      params: operationParams(path),
    });
    entries.value = page.items;
    cwd.value = path;
  } catch (e) {
    listError.value = (e as Error).message;
  } finally {
    loadingList.value = false;
  }
}

async function selectEntry(entry: FileEntry): Promise<void> {
  selected.value = entry;
  content.value = null;
  if (entry.isDir) return;
  if (!readRouteId.value) return;
  loadingContent.value = true;
  try {
    content.value = await fetchDoc<FileContent>(
      props.connectionId,
      {
        routeId: readRouteId.value,
        params: operationParams(entry.path),
      },
      operationCtx.value,
    );
  } finally {
    loadingContent.value = false;
  }
}

async function openEntry(entry: FileEntry): Promise<void> {
  if (entry.isDir) await loadList(entry.path);
  else await selectEntry(entry);
}

function notifySuccess(detail: string): void {
  toast.add({ severity: "success", summary: "Files", detail, life: 2200 });
}

function notifyError(e: unknown): void {
  toast.add({
    severity: "error",
    summary: "File operation failed",
    detail: (e as Error).message,
    life: 4000,
  });
}

async function upload(event: FileUploadUploaderEvent): Promise<void> {
  const routeId = uploadRouteId.value;
  if (!routeId) return;
  const files = Array.isArray(event.files) ? event.files : [event.files];
  if (files.length === 0) return;
  mutating.value = true;
  try {
    await uploadFiles(
      props.connectionId,
      routeId,
      operationCtx.value,
      files,
      operationParams(cwd.value),
      uploadFieldName.value,
    );
    notifySuccess(
      files.length === 1
        ? "Uploaded 1 file."
        : `Uploaded ${files.length} files.`,
    );
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
  }
}

async function createFolder(): Promise<void> {
  const routeId = mkdirRouteId.value;
  const name = newFolderName.value.trim();
  if (!routeId || !name) return;
  mutating.value = true;
  try {
    await runAction(
      props.connectionId,
      routeId,
      operationCtx.value,
      { name },
      operationParams(cwd.value),
    );
    mkdirOpen.value = false;
    newFolderName.value = "";
    notifySuccess("Folder created.");
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
  }
}

function beginRename(): void {
  if (!selected.value) return;
  renameName.value = selected.value.name;
  renameOpen.value = true;
}

async function renameEntry(): Promise<void> {
  const routeId = renameRouteId.value;
  const entry = selected.value;
  const name = renameName.value.trim();
  if (!routeId || !entry || !name || name === entry.name) return;
  mutating.value = true;
  try {
    await runAction(
      props.connectionId,
      routeId,
      operationCtx.value,
      { name },
      operationParams(entry.path),
      "PATCH",
    );
    renameOpen.value = false;
    notifySuccess("Renamed.");
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
  }
}

async function deleteEntry(): Promise<void> {
  const routeId = deleteRouteId.value;
  const entry = selected.value;
  if (!routeId || !entry) return;
  mutating.value = true;
  try {
    await runAction(
      props.connectionId,
      routeId,
      operationCtx.value,
      { path: entry.path },
      operationParams(entry.path),
      "DELETE",
    );
    deleteOpen.value = false;
    notifySuccess("Deleted.");
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
  }
}

watch(
  () => [props.connectionId, props.source?.routeId, startPath.value],
  () => loadList(startPath.value),
  { immediate: true },
);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center gap-1 overflow-x-auto border-b border-surface-200 px-3 py-2 text-sm dark:border-surface-800"
    >
      <template v-for="(c, i) in crumbs" :key="c.path">
        <span v-if="i > 0" class="text-surface-300">/</span>
        <button
          type="button"
          class="rounded px-1.5 py-0.5 text-surface-500 hover:bg-surface-100 hover:text-surface-800 dark:hover:bg-surface-800"
          @click="loadList(c.path)"
        >
          {{ c.label }}
        </button>
      </template>
    </div>

    <div
      class="flex min-h-12 flex-wrap items-center gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <FileUpload
        v-if="canUpload"
        mode="basic"
        :name="uploadFieldName"
        :multiple="multipleUpload"
        :max-file-size="fileConfig?.maxUploadBytes"
        custom-upload
        auto
        choose-label="Upload"
        :disabled="mutating"
        @uploader="upload"
      />
      <Button
        v-if="canMkdir"
        type="button"
        :disabled="mutating"
        label="New folder"
        @click="mkdirOpen = true"
      />
      <Button
        v-if="writable && renameRouteId"
        type="button"
        :disabled="!canRename || mutating"
        label="Rename"
        @click="beginRename"
      />
      <Button
        v-if="writable && deleteRouteId"
        type="button"
        :disabled="!canDelete || mutating"
        label="Delete"
        @click="deleteOpen = true"
      />
      <Button
        v-if="downloadHref"
        as="a"
        :href="downloadHref"
        :download="selected?.name"
        label="Download"
      />
      <Button
        type="button"
        :disabled="loadingList"
        label="Refresh"
        @click="loadList(cwd)"
      />
    </div>

    <div class="flex min-h-0 flex-1">
      <div
        class="w-72 shrink-0 overflow-y-auto border-r border-surface-200 dark:border-surface-800"
      >
        <p v-if="loadingList" class="p-3 text-sm text-surface-400">Loading…</p>
        <p v-else-if="listError" class="p-3 text-sm text-red-500">
          {{ listError }}
        </p>
        <ul v-else>
          <li
            v-for="entry in sorted"
            :key="entry.path"
            class="flex items-stretch"
          >
            <button
              type="button"
              class="flex min-w-0 flex-1 items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-800"
              :class="
                selected?.path === entry.path
                  ? 'bg-surface-100 dark:bg-surface-800'
                  : ''
              "
              @click="selectEntry(entry)"
              @dblclick="openEntry(entry)"
            >
              <AppIcon
                :icon="{ type: 'name', value: entry.isDir ? 'folder' : 'code' }"
                :size="15"
                class="shrink-0 text-surface-400"
              />
              <span
                class="flex-1 truncate text-surface-700 dark:text-surface-200"
                >{{ entry.name }}</span
              >
              <span v-if="!entry.isDir" class="text-xs text-surface-400">{{
                formatBytes(entry.size)
              }}</span>
            </button>
            <Button
              v-if="entry.isDir"
              type="button"
              :aria-label="`Open ${entry.name}`"
              :pt="{
                root: 'w-8 shrink-0 rounded-none px-0 text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800',
              }"
              @click="openEntry(entry)"
            >
              <AppIcon :icon="{ type: 'name', value: 'chevron-right' }" />
            </Button>
          </li>
        </ul>
      </div>

      <div class="min-w-0 flex-1">
        <FilePreview
          :name="selected?.name ?? ''"
          :content="content"
          :loading="loadingContent"
        />
      </div>
    </div>

    <Dialog v-model:visible="mkdirOpen" modal header="New Folder">
      <form class="flex flex-col gap-4" @submit.prevent="createFolder">
        <InputText
          v-model="newFolderName"
          autofocus
          placeholder="Folder name"
        />
        <div class="flex justify-end gap-2">
          <Button
            type="button"
            label="Cancel"
            :disabled="mutating"
            @click="mkdirOpen = false"
          />
          <Button type="submit" label="Create" :disabled="mutating" />
        </div>
      </form>
    </Dialog>

    <Dialog v-model:visible="renameOpen" modal header="Rename">
      <form class="flex flex-col gap-4" @submit.prevent="renameEntry">
        <InputText v-model="renameName" autofocus placeholder="Name" />
        <div class="flex justify-end gap-2">
          <Button
            type="button"
            label="Cancel"
            :disabled="mutating"
            @click="renameOpen = false"
          />
          <Button type="submit" label="Rename" :disabled="mutating" />
        </div>
      </form>
    </Dialog>

    <Dialog v-model:visible="deleteOpen" modal header="Delete">
      <p class="mb-4 text-sm text-surface-600 dark:text-surface-300">
        Delete {{ selected?.name }}?
      </p>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          :disabled="mutating"
          @click="deleteOpen = false"
        />
        <Button
          type="button"
          label="Delete"
          :disabled="mutating"
          @click="deleteEntry"
        />
      </div>
    </Dialog>
  </div>
</template>
