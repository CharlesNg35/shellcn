<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useDropZone } from "@vueuse/core";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import InputText from "primevue/inputtext";
import { useToast } from "primevue/usetoast";
import {
  fetchDoc,
  fetchPage,
  routeURL,
  runAction,
  uploadFiles,
} from "../../api/dataSource";
import type { UploadProgress } from "../../api/dataSource";
import type {
  FileBrowserConfig,
  FileContent,
  FileEntry,
  Page,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import AppIcon from "../../components/AppIcon.vue";
import FileCrumbs from "./FileCrumbs.vue";
import FileEntryGrid from "./FileEntryGrid.vue";
import FileEntryList from "./FileEntryList.vue";
import FilePane from "./FilePane.vue";
import FileToolbar from "./FileToolbar.vue";
import {
  formatBytes,
  languageFor,
  sortEntries,
  viewerFor,
  type FileSortKey,
} from "./fileTypes";
import { dialogRoot } from "../../primevue/preset";

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
const contentError = ref<string | null>(null);
const mutating = ref(false);
const operation = ref<"upload" | "save" | "mkdir" | "rename" | "delete" | null>(
  null,
);
const uploadProgress = ref<UploadProgress | null>(null);
const uploadLabel = ref("");
const mkdirOpen = ref(false);
const renameOpen = ref(false);
const deleteOpen = ref(false);
const previewOpen = ref(false);
const viewMode = ref<"split" | "grid">("split");
const newFolderName = ref("");
const renameName = ref("");

type FileListPage = Page<FileEntry> & { path?: string };

const fileFilter = ref("");
const sortKey = ref<FileSortKey>("name");
const sortDir = ref<"asc" | "desc">("asc");

const sorted = computed(() =>
  sortEntries(entries.value, sortKey.value, sortDir.value),
);

const filtered = computed(() => {
  const term = fileFilter.value.trim().toLowerCase();
  if (!term) return sorted.value;
  return sorted.value.filter((e) => e.name.toLowerCase().includes(term));
});

const listEmptyText = computed(() =>
  fileFilter.value.trim()
    ? "No items match your filter."
    : "This folder is empty.",
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
  () =>
    Boolean(canEdit.value) &&
    editContent.value !== (content.value?.content ?? ""),
);
const selectedEditorLanguage = computed(() =>
  languageFor(selected.value?.name ?? ""),
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
const streamSrc = computed(() => {
  const entry = selected.value;
  if (!entry || entry.isDir || !downloadRouteId.value) return "";
  return routeURL(
    props.connectionId,
    downloadRouteId.value,
    operationCtx.value,
    {
      ...operationParams(entry.path),
      inline: "1",
    },
  );
});

const panelEl = ref<HTMLElement | null>(null);
const { isOverDropZone } = useDropZone(panelEl, {
  onDrop: (files) => {
    if (files?.length && canUpload.value) void uploadFileList(files);
  },
  multiple: multipleUpload.value,
});
const dropActive = computed(() => isOverDropZone.value && canUpload.value);

// Submit is gated so a no-op (empty, or a rename to the same name) can't be
// triggered — the disabled button signals "nothing to apply".
const canSubmitMkdir = computed(
  () => Boolean(newFolderName.value.trim()) && !mutating.value,
);
const canSubmitRename = computed(() => {
  const name = renameName.value.trim();
  return Boolean(name) && name !== selected.value?.name && !mutating.value;
});

const statusLabel = computed(() => {
  if (uploadProgress.value) return `Uploading ${uploadLabel.value}`;
  if (operation.value === "save") return "Saving file";
  if (operation.value === "mkdir") return "Creating folder";
  if (operation.value === "rename") return "Renaming item";
  if (operation.value === "delete") return "Deleting item";
  if (loadingList.value) return "Loading folder";
  return "";
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
  contentError.value = null;
  fileFilter.value = "";
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
  contentError.value = null;
  editContent.value = "";
  if (entry.isDir) return;
  // Media streams via the download URL; only text/unknown fetch inline content.
  const viewer = viewerFor(entry.name, entry.mime);
  if (["image", "pdf", "audio", "video"].includes(viewer)) {
    content.value = { path: entry.path, mime: entry.mime, size: entry.size };
    return;
  }
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
  } catch (e) {
    contentError.value = (e as Error).message;
  } finally {
    loadingContent.value = false;
  }
}

async function saveFile(): Promise<void> {
  const routeId = writeRouteId.value;
  const entry = selected.value;
  if (!routeId || !entry || !dirty.value) return;
  mutating.value = true;
  operation.value = "save";
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
    operation.value = null;
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

function upload(event: FileUploadUploaderEvent): void {
  const files = Array.isArray(event.files) ? event.files : [event.files];
  void uploadFileList(files);
}

async function uploadFileList(files: File[]): Promise<void> {
  const routeId = uploadRouteId.value;
  if (!routeId || files.length === 0) return;
  const total = files.reduce((sum, file) => sum + file.size, 0);
  uploadLabel.value =
    files.length === 1
      ? (files[0]?.name ?? "file")
      : `${files.length} files (${formatBytes(total)})`;
  uploadProgress.value = {
    loaded: 0,
    total,
    percent: 0,
    indeterminate: total <= 0,
  };
  mutating.value = true;
  operation.value = "upload";
  try {
    await uploadFiles(
      props.connectionId,
      routeId,
      operationCtx.value,
      files,
      operationParams(cwd.value),
      uploadFieldName.value,
      {
        onProgress: (progress) => {
          uploadProgress.value = progress;
        },
      },
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
    operation.value = null;
    uploadProgress.value = null;
    uploadLabel.value = "";
  }
}

async function createFolder(): Promise<void> {
  const routeId = mkdirRouteId.value;
  const name = newFolderName.value.trim();
  if (!routeId || !name) return;
  mutating.value = true;
  operation.value = "mkdir";
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
    operation.value = null;
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
  operation.value = "rename";
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
    operation.value = null;
  }
}

async function deleteEntry(): Promise<void> {
  const routeId = deleteRouteId.value;
  const entry = selected.value;
  if (!routeId || !entry) return;
  mutating.value = true;
  operation.value = "delete";
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
    operation.value = null;
  }
}

async function retryContent(): Promise<void> {
  if (selected.value) await selectEntry(selected.value);
}

watch(
  () => [props.connectionId, props.source?.routeId, startPath.value],
  () => loadList(startPath.value),
  { immediate: true },
);
</script>

<template>
  <div ref="panelEl" class="relative flex h-full flex-col">
    <div
      v-if="dropActive"
      class="pointer-events-none absolute inset-0 z-10 m-2 flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-primary-400 bg-primary-50/80 text-primary-700 dark:border-primary-500 dark:bg-primary-950/70 dark:text-primary-200"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'upload-cloud' }" :size="32" />
      <p class="text-sm font-medium">Drop files to upload here</p>
      <p class="text-xs opacity-80">{{ cwd }}</p>
    </div>

    <FileCrumbs :path="cwd" @navigate="loadList" />

    <FileToolbar
      v-model:view-mode="viewMode"
      v-model:filter="fileFilter"
      v-model:sort-key="sortKey"
      v-model:sort-dir="sortDir"
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
      :status-label="statusLabel"
      :upload-progress="uploadProgress"
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
          :entries="filtered"
          :selected-path="selected?.path"
          :loading="loadingList"
          :error="listError"
          :empty-text="listEmptyText"
          @select="selectEntry"
          @open="openEntry"
          @retry="loadList(cwd)"
        />
      </div>

      <div class="min-w-0 flex-1">
        <FilePane
          v-model:edit-content="editContent"
          :selected="selected"
          :content="content"
          :can-edit="Boolean(canEdit)"
          :stream-src="streamSrc"
          :language="selectedEditorLanguage"
          :loading="loadingContent"
          :error="contentError"
          :mutating="mutating"
          :saving="operation === 'save'"
          :dirty="dirty"
          :download-href="downloadHref"
          @save="saveFile"
          @retry="retryContent"
        />
      </div>
    </div>

    <FileEntryGrid
      v-else
      class="min-h-0 flex-1"
      :entries="filtered"
      :selected-path="selected?.path"
      :loading="loadingList"
      :error="listError"
      :empty-text="listEmptyText"
      @select="selectEntry"
      @open="openEntry"
      @retry="loadList(cwd)"
    />

    <Dialog
      v-model:visible="previewOpen"
      modal
      :header="selected?.name ?? 'Preview'"
      :pt="{
        root: dialogRoot('max-w-5xl'),
        content: 'min-h-0 overflow-hidden p-0',
      }"
    >
      <div class="h-[70vh] min-h-0">
        <FilePane
          v-model:edit-content="editContent"
          :selected="selected"
          :content="content"
          :can-edit="Boolean(canEdit)"
          :stream-src="streamSrc"
          :language="selectedEditorLanguage"
          :loading="loadingContent"
          :error="contentError"
          :mutating="mutating"
          :saving="operation === 'save'"
          :dirty="dirty"
          :download-href="downloadHref"
          @save="saveFile"
          @retry="retryContent"
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
          <Button
            type="submit"
            label="Create"
            :loading="operation === 'mkdir'"
            :disabled="!canSubmitMkdir"
          />
        </div>
      </form>
    </Dialog>

    <Dialog v-model:visible="renameOpen" modal header="Rename">
      <form class="flex flex-col gap-4" @submit.prevent="renameEntry">
        <InputText
          v-model="renameName"
          autofocus
          placeholder="Name"
          @focus="($event.target as HTMLInputElement).select()"
        />
        <div class="flex justify-end gap-2">
          <Button
            type="button"
            label="Cancel"
            severity="secondary"
            outlined
            :disabled="mutating"
            @click="renameOpen = false"
          />
          <Button
            type="submit"
            label="Rename"
            :loading="operation === 'rename'"
            :disabled="!canSubmitRename"
          />
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
          :loading="operation === 'delete'"
          :disabled="mutating"
          @click="deleteEntry"
        />
      </div>
    </Dialog>
  </div>
</template>
