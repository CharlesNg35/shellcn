<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import InputText from "primevue/inputtext";
import Textarea from "primevue/textarea";
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
  Page,
} from "../types/projection";
import type { PanelProps } from "./types";
import FileCrumbs from "./file/FileCrumbs.vue";
import FileEntryGrid from "./file/FileEntryGrid.vue";
import FileEntryList from "./file/FileEntryList.vue";
import FilePreview from "./file/FilePreview.vue";
import FileToolbar from "./file/FileToolbar.vue";

const props = defineProps<PanelProps>();
const toast = useToast();

const fileConfig = computed(
  () => props.config as FileBrowserConfig | undefined,
);
const pathParam = computed(() => fileConfig.value?.pathParam ?? "path");
const readRouteId = computed(() => fileConfig.value?.readRouteId);
const downloadRouteId = computed(() => fileConfig.value?.downloadRouteId);
const writeRouteId = computed(() => fileConfig.value?.writeRouteId);
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
  () => props.source?.params?.[pathParam.value] ?? ".",
);
const cwd = ref(startPath.value);
const entries = ref<FileEntry[]>([]);
const loadingList = ref(false);
const listError = ref<string | null>(null);

const selected = ref<FileEntry | null>(null);
const content = ref<FileContent | null>(null);
const editContent = ref("");
const loadingContent = ref(false);
const mutating = ref(false);
const mkdirOpen = ref(false);
const renameOpen = ref(false);
const deleteOpen = ref(false);
const previewOpen = ref(false);
const viewMode = ref<"split" | "grid">("split");
const newFolderName = ref("");
const renameName = ref("");

type FileListPage = Page<FileEntry> & { path?: string };

const sorted = computed(() =>
  [...entries.value].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.localeCompare(b.name);
  }),
);

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
const canEdit = computed(
  () =>
    writable.value &&
    Boolean(writeRouteId.value) &&
    content.value?.encoding === "utf8" &&
    selected.value &&
    !selected.value.isDir,
);
const dirty = computed(
  () => canEdit.value && editContent.value !== (content.value?.content ?? ""),
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

function parentPath(path: string): string {
  const normalized = path.replace(/\/+$/, "");
  if (!normalized || normalized === "/") return "/";
  const idx = normalized.lastIndexOf("/");
  return idx <= 0 ? "/" : normalized.slice(0, idx);
}

function resolvedListPath(requested: string, page: FileListPage): string {
  if (page.path) return page.path;
  const first = page.items[0]?.path;
  return first?.startsWith("/") ? parentPath(first) : requested;
}

async function loadList(path: string): Promise<void> {
  if (!props.source) return;
  loadingList.value = true;
  listError.value = null;
  selected.value = null;
  content.value = null;
  try {
    const page = (await fetchPage<FileEntry>(
      props.connectionId,
      {
        routeId: props.source.routeId,
        params: operationParams(path),
      },
      operationCtx.value,
    )) as FileListPage;
    entries.value = page.items;
    cwd.value = resolvedListPath(path, page);
  } catch (e) {
    listError.value = (e as Error).message;
  } finally {
    loadingList.value = false;
  }
}

async function selectEntry(entry: FileEntry): Promise<void> {
  selected.value = entry;
  content.value = null;
  editContent.value = "";
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
    editContent.value = content.value.content ?? "";
  } finally {
    loadingContent.value = false;
  }
}

async function saveFile(): Promise<void> {
  const routeId = writeRouteId.value;
  const entry = selected.value;
  if (!routeId || !entry || !dirty.value) return;
  mutating.value = true;
  try {
    await runAction(
      props.connectionId,
      routeId,
      operationCtx.value,
      { content: editContent.value },
      operationParams(entry.path),
      "PUT",
    );
    if (content.value) {
      content.value = {
        ...content.value,
        content: editContent.value,
        size: editContent.value.length,
        truncated: false,
      };
    }
    notifySuccess("Saved.");
    await loadList(cwd.value);
    const updated = entries.value.find((e) => e.path === entry.path);
    if (updated) await selectEntry(updated);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
  }
}

async function openEntry(entry: FileEntry): Promise<void> {
  if (entry.isDir) await loadList(entry.path);
  else {
    await selectEntry(entry);
    if (viewMode.value === "grid") previewOpen.value = true;
  }
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
    <FileCrumbs :path="cwd" @navigate="loadList" />

    <FileToolbar
      v-model:view-mode="viewMode"
      :can-upload="canUpload"
      :can-mkdir="canMkdir"
      :can-rename="canRename"
      :can-delete="canDelete"
      :can-show-rename="writable && Boolean(renameRouteId)"
      :can-show-delete="writable && Boolean(deleteRouteId)"
      :download-href="downloadHref"
      :download-name="selected?.name"
      :multiple-upload="multipleUpload"
      :max-upload-bytes="fileConfig?.maxUploadBytes"
      :upload-field-name="uploadFieldName"
      :mutating="mutating"
      :loading="loadingList"
      @upload="upload"
      @mkdir="mkdirOpen = true"
      @rename="beginRename"
      @delete="deleteOpen = true"
      @refresh="loadList(cwd)"
    />

    <div v-if="viewMode === 'split'" class="flex min-h-0 flex-1">
      <div
        class="w-80 shrink-0 border-r border-surface-200 bg-surface-50/40 dark:border-surface-800 dark:bg-surface-950/30"
      >
        <FileEntryList
          :entries="sorted"
          :selected-path="selected?.path"
          :loading="loadingList"
          :error="listError"
          @select="selectEntry"
          @open="openEntry"
        />
      </div>

      <div class="min-w-0 flex-1">
        <div v-if="canEdit" class="flex h-full flex-col">
          <div
            class="flex items-center justify-end border-b border-surface-200 px-3 py-2 dark:border-surface-800"
          >
            <Button
              type="button"
              label="Save"
              :disabled="!dirty || mutating"
              @click="saveFile"
            />
          </div>
          <Textarea
            v-model="editContent"
            class="h-full min-h-0 w-full flex-1 resize-none rounded-none border-0 p-4 font-mono text-xs leading-relaxed"
            spellcheck="false"
            :disabled="mutating"
          />
        </div>
        <FilePreview
          v-else
          :name="selected?.name ?? ''"
          :content="content"
          :loading="loadingContent"
        />
      </div>
    </div>

    <FileEntryGrid
      v-else
      class="min-h-0 flex-1"
      :entries="sorted"
      :selected-path="selected?.path"
      :loading="loadingList"
      :error="listError"
      @select="selectEntry"
      @open="openEntry"
    />

    <Dialog
      v-model:visible="previewOpen"
      modal
      :header="selected?.name ?? 'Preview'"
      :pt="{
        root: 'w-full max-w-5xl overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-2xl dark:border-surface-800 dark:bg-surface-900',
      }"
    >
      <div class="h-[70vh] min-h-0">
        <div v-if="canEdit" class="flex h-full flex-col">
          <div
            class="flex items-center justify-end border-b border-surface-200 px-3 py-2 dark:border-surface-800"
          >
            <Button
              type="button"
              label="Save"
              :disabled="!dirty || mutating"
              @click="saveFile"
            />
          </div>
          <Textarea
            v-model="editContent"
            class="h-full min-h-0 w-full flex-1 resize-none rounded-none border-0 p-4 font-mono text-xs leading-relaxed"
            spellcheck="false"
            :disabled="mutating"
          />
        </div>
        <FilePreview
          v-else
          :name="selected?.name ?? ''"
          :content="content"
          :loading="loadingContent"
        />
      </div>
    </Dialog>

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
            severity="secondary"
            outlined
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
            severity="secondary"
            outlined
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
          severity="secondary"
          outlined
          :disabled="mutating"
          @click="deleteOpen = false"
        />
        <Button
          type="button"
          label="Delete"
          severity="danger"
          :disabled="mutating"
          @click="deleteEntry"
        />
      </div>
    </Dialog>
  </div>
</template>
