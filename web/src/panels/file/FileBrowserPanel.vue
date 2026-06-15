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
} from "@/api/dataSource";
import type { UploadProgress } from "@/api/dataSource";
import { apiFetch } from "@/api/client";
import {
  FileTransferOperation,
  type FileBrowserConfig,
  type FileContent,
  type FileEntry,
  type Page,
} from "@/types/projection";
import type { PanelProps } from "../core/types";
import AppAlert from "@/components/AppAlert.vue";
import AppIcon from "@/components/AppIcon.vue";
import FileCrumbs from "./FileCrumbs.vue";
import FileEntryGrid from "./FileEntryGrid.vue";
import FileEntryList from "./FileEntryList.vue";
import FilePane from "./FilePane.vue";
import FileSelectionBar from "./FileSelectionBar.vue";
import FileToolbar from "./FileToolbar.vue";
import FileTransferDialog from "./FileTransferDialog.vue";
import {
  formatBytes,
  languageFor,
  sortEntries,
  viewerFor,
  type FileSortKey,
} from "./fileTypes";
import { dialogRoot } from "@/primevue/preset";

const props = defineProps<PanelProps>();
const toast = useToast();

const fileConfig = computed(
  () => props.config as FileBrowserConfig | undefined,
);
const pathParam = computed(() => fileConfig.value?.pathParam ?? "path");
const routes = computed(() => fileConfig.value?.routes);
const uploadConfig = computed(() => fileConfig.value?.upload);
const readRouteId = computed(() => routes.value?.read);
const downloadRouteId = computed(() => routes.value?.download);
const writeRouteId = computed(() => routes.value?.write);
const uploadRouteId = computed(() => uploadConfig.value?.routeId);
const mkdirRouteId = computed(() => routes.value?.mkdir);
const renameRouteId = computed(() => routes.value?.rename);
const deleteRouteId = computed(() => routes.value?.delete);
const chmodRouteId = computed(() => routes.value?.chmod);
const archiveRouteId = computed(() => routes.value?.archive);
const transferConfig = computed(() => fileConfig.value?.transfer);
const writable = computed(() => Boolean(fileConfig.value?.writable));
const multipleUpload = computed(() => uploadConfig.value?.multiple ?? true);
const uploadFieldName = computed(
  () => uploadConfig.value?.fieldName ?? "files",
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
type FilePanelOperation =
  | "upload"
  | "save"
  | "mkdir"
  | "rename"
  | "delete"
  | "chmod"
  | FileTransferOperation
  | null;
const operation = ref<FilePanelOperation>(null);

const selectedPaths = ref<Set<string>>(new Set());
const chmodOpen = ref(false);
const bulkDeleteOpen = ref(false);
const transferOpen = ref(false);
const transferOperation = ref<FileTransferOperation>(
  FileTransferOperation.Copy,
);
const destPath = ref("");
const chmodMode = ref("");
const uploadProgress = ref<UploadProgress | null>(null);
const uploadLabel = ref("");
const uploadWarning = ref("");
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

const operationCtx = computed(() => ({
  resource: props.resource,
  record: props.record,
}));
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

const selectionCount = computed(() => selectedPaths.value.size);
const hasSelection = computed(() => selectionCount.value > 0);
const selectedEntries = computed(() =>
  filtered.value.filter((e) => selectedPaths.value.has(e.path)),
);
const allSelected = computed(
  () =>
    filtered.value.length > 0 &&
    filtered.value.every((e) => selectedPaths.value.has(e.path)),
);
const canBulkDelete = computed(
  () => writable.value && Boolean(deleteRouteId.value),
);
const canMove = computed(
  () => writable.value && supportsTransfer(FileTransferOperation.Move),
);
const canCopy = computed(
  () => writable.value && supportsTransfer(FileTransferOperation.Copy),
);
const canChmod = computed(() => writable.value && Boolean(chmodRouteId.value));
const canArchive = computed(
  () =>
    Boolean(archiveRouteId.value) ||
    supportsTransfer(FileTransferOperation.Archive),
);
const selectable = computed(
  () =>
    canBulkDelete.value ||
    canMove.value ||
    canCopy.value ||
    canChmod.value ||
    canArchive.value,
);
const validMode = computed(() => /^[0-7]{3,4}$/.test(chmodMode.value.trim()));
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
  uploadWarning.value = "";
  selected.value = null;
  selectedPaths.value = new Set();
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

function notifyUploadWarning(detail: string): void {
  uploadWarning.value = detail;
}

function uploadSizeWarning(files: File[], maxBytes: number): string {
  const names = files
    .slice(0, 3)
    .map((file) => file.name)
    .join(", ");
  const suffix = files.length > 3 ? ` and ${files.length - 3} more` : "";
  if (files.length === 1) {
    const file = files[0]!;
    return `${file.name} is ${formatBytes(file.size)}. Maximum upload size is ${formatBytes(maxBytes)}.`;
  }
  return `${files.length} files exceed the ${formatBytes(maxBytes)} upload limit: ${names}${suffix}.`;
}

function validUploadFiles(files: File[]): File[] | null {
  const maxBytes = uploadConfig.value?.maxBytes ?? 0;
  if (maxBytes <= 0) return files;
  const oversized = files.filter((file) => file.size > maxBytes);
  if (!oversized.length) return files;
  notifyUploadWarning(uploadSizeWarning(oversized, maxBytes));
  return null;
}

function upload(event: FileUploadUploaderEvent): void {
  const files = Array.isArray(event.files) ? event.files : [event.files];
  void uploadFileList(files);
}

async function uploadFileList(files: File[]): Promise<void> {
  const routeId = uploadRouteId.value;
  if (!routeId || files.length === 0) return;
  const validFiles = validUploadFiles(files);
  if (!validFiles) return;
  uploadWarning.value = "";
  const total = files.reduce((sum, file) => sum + file.size, 0);
  uploadLabel.value =
    validFiles.length === 1
      ? (validFiles[0]?.name ?? "file")
      : `${validFiles.length} files (${formatBytes(total)})`;
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
      validFiles,
      operationParams(cwd.value),
      uploadFieldName.value,
      {
        onProgress: (progress) => {
          uploadProgress.value = progress;
        },
      },
    );
    notifySuccess(
      validFiles.length === 1
        ? "Uploaded 1 file."
        : `Uploaded ${validFiles.length} files.`,
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

function toggleSelect(entry: FileEntry): void {
  const next = new Set(selectedPaths.value);
  if (next.has(entry.path)) next.delete(entry.path);
  else next.add(entry.path);
  selectedPaths.value = next;
}

function toggleSelectAll(): void {
  if (allSelected.value) {
    selectedPaths.value = new Set();
    return;
  }
  selectedPaths.value = new Set(filtered.value.map((e) => e.path));
}

function clearSelection(): void {
  selectedPaths.value = new Set();
}

function supportsTransfer(kind: FileTransferOperation): boolean {
  const transfer = transferConfig.value;
  return Boolean(
    transfer?.source?.routeId && (transfer.operations ?? []).includes(kind),
  );
}

function beginStreamTransfer(kind: FileTransferOperation): void {
  transferOperation.value = kind;
  transferOpen.value = true;
}

function beginMove(): void {
  destPath.value = cwd.value;
  beginStreamTransfer(FileTransferOperation.Move);
}

function beginCopy(): void {
  destPath.value = cwd.value;
  beginStreamTransfer(FileTransferOperation.Copy);
}

function beginChmod(): void {
  chmodMode.value = "0644";
  chmodOpen.value = true;
}

async function bulkDelete(): Promise<void> {
  const routeId = deleteRouteId.value;
  const paths = selectedEntries.value.map((e) => e.path);
  if (!routeId || paths.length === 0) return;
  mutating.value = true;
  operation.value = "delete";
  try {
    for (const path of paths) {
      await runAction(
        props.connectionId,
        routeId,
        operationCtx.value,
        { path },
        operationParams(path),
        "DELETE",
      );
    }
    bulkDeleteOpen.value = false;
    clearSelection();
    notifySuccess(
      paths.length === 1 ? "Deleted." : `Deleted ${paths.length} items.`,
    );
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
    operation.value = null;
  }
}

async function bulkChmod(): Promise<void> {
  const routeId = chmodRouteId.value;
  const mode = chmodMode.value.trim();
  const paths = selectedEntries.value.map((e) => e.path);
  if (!routeId || !validMode.value || paths.length === 0) return;
  mutating.value = true;
  operation.value = "chmod";
  try {
    await runAction(
      props.connectionId,
      routeId,
      operationCtx.value,
      { paths, mode },
      props.source?.params,
    );
    chmodOpen.value = false;
    clearSelection();
    notifySuccess("Permissions updated.");
    await loadList(cwd.value);
  } catch (e) {
    notifyError(e);
  } finally {
    mutating.value = false;
    operation.value = null;
  }
}

function archiveSelected(): void {
  const routeId = archiveRouteId.value;
  const paths = selectedEntries.value.map((e) => e.path);
  if (paths.length === 0) return;
  if (supportsTransfer(FileTransferOperation.Archive)) {
    beginStreamTransfer(FileTransferOperation.Archive);
    return;
  }
  if (!routeId) return;
  operation.value = FileTransferOperation.Archive;
  mutating.value = true;
  downloadArchive(routeId, paths)
    .then(() => {
      notifySuccess("Archive ready.");
    })
    .catch((e) => notifyError(e))
    .finally(() => {
      mutating.value = false;
      operation.value = null;
    });
}

function onTransferComplete(): void {
  if (transferOperation.value !== FileTransferOperation.Archive)
    clearSelection();
  void loadList(cwd.value);
}

async function downloadArchive(
  routeId: string,
  paths: string[],
): Promise<void> {
  const url = routeURL(
    props.connectionId,
    routeId,
    operationCtx.value,
    props.source?.params ?? {},
  );
  const res = await apiFetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ paths }),
  });
  const blob = await res.blob();
  const href = URL.createObjectURL(blob);
  const name =
    paths.length === 1 ? `${baseName(paths[0]!)}.zip` : "archive.zip";
  const a = document.createElement("a");
  a.href = href;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(href);
}

function baseName(path: string): string {
  const trimmed = path.replace(/\/+$/, "");
  const idx = trimmed.lastIndexOf("/");
  return idx >= 0 ? trimmed.slice(idx + 1) : trimmed;
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
      :can-show-delete="
        writable && Boolean(deleteRouteId) && !(selectable && hasSelection)
      "
      :download-href="downloadHref"
      :download-name="selected?.name"
      :multiple-upload="multipleUpload"
      :max-upload-bytes="uploadConfig?.maxBytes"
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

    <div
      v-if="uploadWarning"
      class="border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <AppAlert
        tone="warning"
        title="Upload blocked"
        closable
        @close="uploadWarning = ''"
      >
        {{ uploadWarning }}
      </AppAlert>
    </div>

    <FileSelectionBar
      v-if="selectable && hasSelection"
      :count="selectionCount"
      :all-selected="allSelected"
      :some-selected="hasSelection"
      :can-move="canMove"
      :can-copy="canCopy"
      :can-chmod="canChmod"
      :can-archive="canArchive"
      :can-delete="canBulkDelete"
      :busy="mutating"
      @toggle-all="toggleSelectAll"
      @clear="clearSelection"
      @move="beginMove"
      @copy="beginCopy"
      @chmod="beginChmod"
      @archive="archiveSelected"
      @delete="bulkDeleteOpen = true"
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
          :selectable="selectable"
          :selected-paths="selectedPaths"
          @select="selectEntry"
          @open="openEntry"
          @retry="loadList(cwd)"
          @toggle="toggleSelect"
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
      :selectable="selectable"
      :selected-paths="selectedPaths"
      @select="selectEntry"
      @open="openEntry"
      @retry="loadList(cwd)"
      @toggle="toggleSelect"
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

    <Dialog v-model:visible="chmodOpen" modal header="Change permissions">
      <form class="flex w-80 flex-col gap-4" @submit.prevent="bulkChmod">
        <p class="text-sm text-surface-600 dark:text-surface-300">
          Set octal mode for {{ selectionCount }}
          {{ selectionCount === 1 ? "item" : "items" }}:
        </p>
        <InputText
          v-model="chmodMode"
          autofocus
          placeholder="0644"
          aria-label="Octal permission mode"
          :invalid="Boolean(chmodMode.trim()) && !validMode"
        />
        <small
          v-if="Boolean(chmodMode.trim()) && !validMode"
          class="text-danger-500"
        >
          Enter a 3 or 4 digit octal mode (e.g. 0644).
        </small>
        <div class="flex justify-end gap-2">
          <Button
            type="button"
            label="Cancel"
            severity="secondary"
            outlined
            :disabled="mutating"
            @click="chmodOpen = false"
          />
          <Button
            type="submit"
            label="Apply"
            :loading="operation === 'chmod'"
            :disabled="!validMode || mutating"
          />
        </div>
      </form>
    </Dialog>

    <Dialog v-model:visible="bulkDeleteOpen" modal header="Delete selection">
      <p class="mb-4 w-80 text-sm text-surface-600 dark:text-surface-300">
        Delete {{ selectionCount }}
        {{ selectionCount === 1 ? "item" : "items" }}? This cannot be undone.
      </p>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          outlined
          :disabled="mutating"
          @click="bulkDeleteOpen = false"
        />
        <Button
          type="button"
          label="Delete"
          severity="danger"
          :loading="operation === 'delete'"
          :disabled="mutating"
          @click="bulkDelete"
        />
      </div>
    </Dialog>

    <FileTransferDialog
      v-if="transferConfig?.source && transferOpen"
      v-model:visible="transferOpen"
      :connection-id="props.connectionId"
      :ctx="operationCtx"
      :config="transferConfig"
      :operation="transferOperation"
      :paths="selectedEntries.map((entry) => entry.path)"
      :default-destination="destPath"
      @complete="onTransferComplete"
    />
  </div>
</template>
