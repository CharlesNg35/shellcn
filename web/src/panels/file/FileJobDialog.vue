<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import ProgressBar from "primevue/progressbar";
import Tree from "primevue/tree";
import { useStream } from "@/composables/useStream";
import { fetchPage, type ResolveContext } from "@/api/dataSource";
import {
  FileJobOperation,
  type DataSource,
  type FileEntry,
  type FileJobConfig,
} from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";
import StreamStatusBar from "../streaming/StreamStatusBar.vue";
import { formatBytes } from "./fileTypes";

interface FileJobFrame {
  type?: string;
  jobId?: string;
  status?: string;
  message?: string;
  path?: string;
  operation?: string;
  percent?: number;
  bytesDone?: number;
  bytesTotal?: number;
  filesDone?: number;
  filesTotal?: number;
  rateBps?: number;
  downloadUrl?: string;
  error?: string;
}

interface FolderNode {
  key: string;
  label: string;
  icon: string;
  data: { path: string };
  leaf: boolean;
  loading?: boolean;
  children?: FolderNode[];
}

interface FolderTreeNode {
  data?: { path?: string };
  children?: unknown;
}

const visible = defineModel<boolean>("visible", { default: false });

const props = defineProps<{
  connectionId: string;
  ctx: ResolveContext;
  config: FileJobConfig;
  operation: FileJobOperation;
  paths: string[];
  defaultDestination?: string;
  folderSource?: DataSource;
  pathParam?: string;
}>();

const emit = defineEmits<{
  complete: [];
}>();

const destination = ref(props.defaultDestination ?? "");
const jobId = ref("");
const active = ref(false);
const finished = ref(false);
const failed = ref(false);
const statusText = ref("");
const percent = ref<number | null>(null);
const bytesDone = ref(0);
const bytesTotal = ref(0);
const filesDone = ref(0);
const filesTotal = ref(0);
const rateBps = ref(0);
const downloadUrl = ref("");
const lines = ref<string[]>([]);
const folderNodes = ref<FolderNode[]>([]);
const folderRoot = ref("");
const folderError = ref("");
const folderLoading = ref(false);
const expandedKeys = ref<Record<string, boolean>>({});
const selectedFolderKeys = ref<Record<string, boolean>>({});

const { status, error, send, reconnect } = useStream(
  props.connectionId,
  props.config.source,
  props.ctx,
  handleFrame,
  { keySuffix: "file-job" },
);

const operationLabel = computed(() => {
  switch (props.operation) {
    case FileJobOperation.Move:
      return "Move";
    case FileJobOperation.Copy:
      return "Copy";
    case FileJobOperation.Archive:
      return "Archive";
    case FileJobOperation.Extract:
      return "Extract";
    case FileJobOperation.Sync:
      return "Sync";
    default:
      return "File operation";
  }
});

const itemLabel = computed(
  () => `${props.paths.length} item${props.paths.length === 1 ? "" : "s"}`,
);

const headerText = computed(() => `${operationLabel.value} ${itemLabel.value}`);

const destinationLabel = computed(() => {
  switch (props.operation) {
    case FileJobOperation.Move:
      return "Move to folder";
    case FileJobOperation.Copy:
      return "Copy to folder";
    case FileJobOperation.Extract:
      return "Extract to folder";
    case FileJobOperation.Sync:
      return "Sync with folder";
    default:
      return "Destination folder";
  }
});

const destinationRequired = computed(() =>
  [
    FileJobOperation.Move,
    FileJobOperation.Copy,
    FileJobOperation.Extract,
    FileJobOperation.Sync,
  ].some((operation) => operation === props.operation),
);

const canBrowseFolders = computed(
  () => destinationRequired.value && Boolean(props.folderSource?.routeId),
);

const canStart = computed(
  () =>
    status.value === "open" &&
    props.paths.length > 0 &&
    !active.value &&
    (!destinationRequired.value || Boolean(destination.value.trim())),
);

const parentDisabled = computed(
  () =>
    active.value ||
    !canBrowseFolders.value ||
    parentPath(folderRoot.value) === folderRoot.value,
);

const progressMode = computed(() =>
  percent.value == null ? "indeterminate" : "determinate",
);

const progressValue = computed(() =>
  percent.value == null ? undefined : Math.max(0, Math.min(100, percent.value)),
);

const detailText = computed(() => {
  const parts: string[] = [];
  if (filesTotal.value > 0)
    parts.push(`${filesDone.value}/${filesTotal.value} files`);
  if (bytesTotal.value > 0)
    parts.push(
      `${formatBytes(bytesDone.value)} / ${formatBytes(bytesTotal.value)}`,
    );
  else if (bytesDone.value > 0) parts.push(formatBytes(bytesDone.value));
  if (rateBps.value > 0) parts.push(`${formatBytes(rateBps.value)}/s`);
  return parts.join(" · ");
});

function reset(): void {
  jobId.value = "";
  active.value = false;
  finished.value = false;
  failed.value = false;
  statusText.value = "";
  percent.value = null;
  bytesDone.value = 0;
  bytesTotal.value = 0;
  filesDone.value = 0;
  filesTotal.value = 0;
  rateBps.value = 0;
  downloadUrl.value = "";
  lines.value = [];
}

function makeJobId(): string {
  return (
    globalThis.crypto?.randomUUID?.() ??
    `file-job-${Date.now()}-${Math.random().toString(36).slice(2)}`
  );
}

function startJob(): void {
  if (!canStart.value) return;
  reset();
  jobId.value = makeJobId();
  active.value = true;
  statusText.value = `${operationLabel.value} started`;
  send(
    JSON.stringify({
      type: "start",
      jobId: jobId.value,
      operation: props.operation,
      paths: props.paths,
      destination: destination.value.trim(),
    }),
  );
}

function cancelJob(): void {
  if (!active.value || !jobId.value) return;
  send(JSON.stringify({ type: "cancel", jobId: jobId.value }));
  statusText.value = "Cancel requested";
}

function cleanPath(path: string | undefined): string {
  const trimmed = path?.trim();
  return trimmed || ".";
}

function parentPath(path: string): string {
  const p = cleanPath(path);
  if (p === "." || p === "/") return p;
  const trimmed = p.endsWith("/") ? p.slice(0, -1) : p;
  const index = trimmed.lastIndexOf("/");
  if (index <= 0) return p.startsWith("/") ? "/" : ".";
  return trimmed.slice(0, index);
}

function folderLabel(path: string): string {
  if (path === "/") return "/";
  if (path === ".") return ".";
  const trimmed = path.endsWith("/") ? path.slice(0, -1) : path;
  return trimmed.slice(trimmed.lastIndexOf("/") + 1) || trimmed;
}

function makeFolderNode(path: string, label = folderLabel(path)): FolderNode {
  return {
    key: path,
    label,
    icon: "pi pi-folder",
    data: { path },
    leaf: false,
  };
}

function withFolderParam(path: string): DataSource | undefined {
  if (!props.folderSource) return undefined;
  return {
    ...props.folderSource,
    params: {
      ...props.folderSource.params,
      [props.pathParam ?? "path"]: path,
    },
  };
}

function replaceFolderNode(
  nodes: FolderNode[],
  key: string,
  update: (node: FolderNode) => FolderNode,
): FolderNode[] {
  return nodes.map((node) => {
    if (node.key === key) return update(node);
    if (!node.children) return node;
    return { ...node, children: replaceFolderNode(node.children, key, update) };
  });
}

async function loadFolderChildren(path: string): Promise<void> {
  const source = withFolderParam(path);
  if (!source) return;
  folderError.value = "";
  folderNodes.value = replaceFolderNode(folderNodes.value, path, (node) => ({
    ...node,
    loading: true,
  }));
  try {
    const page = await fetchPage<FileEntry>(
      props.connectionId,
      source,
      props.ctx,
      { limit: 250 },
    );
    const children = page.items
      .filter((entry) => entry.isDir)
      .sort((a, b) => a.name.localeCompare(b.name))
      .map((entry) => makeFolderNode(entry.path, entry.name));
    folderNodes.value = replaceFolderNode(folderNodes.value, path, (node) => ({
      ...node,
      children,
      leaf: children.length === 0,
      loading: false,
    }));
  } catch (e) {
    folderError.value =
      e instanceof Error ? e.message : "Could not load folders.";
    folderNodes.value = replaceFolderNode(folderNodes.value, path, (node) => ({
      ...node,
      loading: false,
    }));
  }
}

async function openFolderRoot(path?: string): Promise<void> {
  if (!canBrowseFolders.value) return;
  const root = cleanPath(path);
  folderRoot.value = root;
  destination.value = root;
  folderError.value = "";
  folderLoading.value = true;
  folderNodes.value = [makeFolderNode(root, root)];
  expandedKeys.value = { [root]: true };
  selectedFolderKeys.value = { [root]: true };
  try {
    await loadFolderChildren(root);
  } finally {
    folderLoading.value = false;
  }
}

function selectFolder(path: string): void {
  destination.value = path;
  selectedFolderKeys.value = { [path]: true };
}

function onFolderSelect(node: FolderTreeNode): void {
  const path = node.data?.path;
  if (path) selectFolder(path);
}

function onFolderExpand(node: FolderTreeNode): void {
  const path = node.data?.path;
  if (path && node.children === undefined) void loadFolderChildren(path);
}

function openParentFolder(): void {
  void openFolderRoot(parentPath(folderRoot.value));
}

function openDefaultFolder(): void {
  void openFolderRoot(props.defaultDestination);
}

function appendLine(line: string): void {
  lines.value = [...lines.value.slice(-80), line];
}

function handleFrame(raw: string): void {
  let frame: FileJobFrame;
  try {
    frame = JSON.parse(raw) as FileJobFrame;
  } catch {
    appendLine(raw);
    return;
  }
  if (frame.jobId && jobId.value && frame.jobId !== jobId.value) return;
  if (frame.message) appendLine(frame.message);
  if (frame.path) statusText.value = frame.path;
  if (frame.status) statusText.value = frame.status;
  if (typeof frame.percent === "number") percent.value = frame.percent;
  if (typeof frame.bytesDone === "number") bytesDone.value = frame.bytesDone;
  if (typeof frame.bytesTotal === "number") bytesTotal.value = frame.bytesTotal;
  if (typeof frame.filesDone === "number") filesDone.value = frame.filesDone;
  if (typeof frame.filesTotal === "number") filesTotal.value = frame.filesTotal;
  if (typeof frame.rateBps === "number") rateBps.value = frame.rateBps;
  if (frame.downloadUrl) downloadUrl.value = frame.downloadUrl;
  if (frame.error) {
    failed.value = true;
    active.value = false;
    statusText.value = frame.error;
    appendLine(frame.error);
  }
  if (frame.type === "complete") {
    active.value = false;
    finished.value = true;
    statusText.value = frame.message || `${operationLabel.value} complete`;
    emit("complete");
  }
  if (frame.type === "error") {
    failed.value = true;
    active.value = false;
  }
}

watch(
  () => [visible.value, props.operation, props.defaultDestination],
  () => {
    if (!visible.value) return;
    destination.value = props.defaultDestination ?? "";
    reset();
    void openFolderRoot(props.defaultDestination);
  },
  { immediate: true },
);
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    :header="headerText"
    :style="{ width: '34rem' }"
    :breakpoints="{ '640px': '94vw' }"
  >
    <div class="space-y-4">
      <StreamStatusBar :status="status" :error="error" @reconnect="reconnect" />

      <div
        class="rounded-md border border-surface-200 p-3 text-sm dark:border-surface-800"
      >
        <div class="font-medium text-surface-900 dark:text-surface-50">
          {{ itemLabel }}
        </div>
        <div
          class="mt-2 max-h-28 space-y-1 overflow-auto text-surface-500 dark:text-surface-400"
        >
          <div v-for="path in paths" :key="path" class="truncate">
            {{ path }}
          </div>
        </div>
      </div>

      <div v-if="destinationRequired" class="space-y-2">
        <div class="flex items-center justify-between gap-3">
          <span
            class="text-xs font-medium tracking-wide text-surface-500 uppercase dark:text-surface-400"
          >
            {{ destinationLabel }}
          </span>
          <div v-if="canBrowseFolders" class="flex items-center gap-1">
            <Button
              type="button"
              size="small"
              severity="secondary"
              outlined
              :disabled="parentDisabled"
              @click="openParentFolder"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'corner-left-up' }"
                :size="14"
              />
              Parent
            </Button>
            <Button
              type="button"
              size="small"
              severity="secondary"
              outlined
              :disabled="active"
              @click="openDefaultFolder"
            >
              <AppIcon :icon="{ type: 'lucide', value: 'folder' }" :size="14" />
              Current
            </Button>
          </div>
        </div>

        <div v-if="canBrowseFolders" class="space-y-2">
          <Tree
            v-model:expanded-keys="expandedKeys"
            v-model:selection-keys="selectedFolderKeys"
            :value="folderNodes"
            selection-mode="single"
            :meta-key-selection="false"
            loading-mode="icon"
            aria-label="Destination folders"
            @node-select="onFolderSelect"
            @node-expand="onFolderExpand"
          />
          <div
            v-if="folderLoading"
            class="text-xs text-surface-500 dark:text-surface-400"
          >
            Loading folders...
          </div>
          <div
            v-if="folderError"
            class="flex items-center justify-between gap-3 rounded-md border border-amber-500/30 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:bg-amber-950/60 dark:text-amber-200"
          >
            <span class="min-w-0 flex-1">{{ folderError }}</span>
            <Button
              type="button"
              size="small"
              severity="warn"
              text
              @click="openFolderRoot(folderRoot)"
            >
              Retry
            </Button>
          </div>
        </div>

        <label class="block space-y-1">
          <span class="text-xs text-surface-500 dark:text-surface-400">
            Selected path
          </span>
          <InputText
            v-model="destination"
            class="w-full"
            aria-label="Job destination"
            :disabled="active"
          />
          <span class="block text-xs text-surface-500 dark:text-surface-400">
            Select a folder above or type a destination path. Each item keeps
            its current name.
          </span>
        </label>
      </div>

      <div v-if="active || finished || failed" class="space-y-2">
        <div class="flex items-center justify-between gap-3 text-sm">
          <span
            class="min-w-0 truncate font-medium text-surface-800 dark:text-surface-100"
          >
            {{ statusText || operationLabel }}
          </span>
          <span
            v-if="detailText"
            class="shrink-0 text-xs text-surface-500 dark:text-surface-400"
          >
            {{ detailText }}
          </span>
        </div>
        <ProgressBar
          :value="progressValue"
          :mode="progressMode"
          :show-value="false"
          aria-label="File job progress"
          style="height: 0.5rem"
        />
      </div>

      <div
        v-if="lines.length"
        class="max-h-36 overflow-auto rounded-md bg-surface-950 p-3 font-mono text-xs text-surface-100"
      >
        <div v-for="(line, index) in lines" :key="index">
          {{ line }}
        </div>
      </div>

      <div class="flex justify-end gap-2">
        <Button
          v-if="downloadUrl"
          as="a"
          severity="secondary"
          :href="downloadUrl"
          download
        >
          <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="15" />
          Download
        </Button>
        <Button
          v-if="active"
          type="button"
          severity="danger"
          outlined
          @click="cancelJob"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
          Cancel
        </Button>
        <Button v-else type="button" :disabled="!canStart" @click="startJob">
          <AppIcon :icon="{ type: 'lucide', value: 'play' }" :size="15" />
          {{ operationLabel }}
        </Button>
      </div>
    </div>
  </Dialog>
</template>
