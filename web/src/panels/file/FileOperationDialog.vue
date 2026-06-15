<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Tree from "primevue/tree";
import { fetchPage, runAction, type ResolveContext } from "@/api/dataSource";
import {
  FileOperation,
  type DataSource,
  type FileEntry,
} from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";

interface FolderNode {
  key: string;
  label: string;
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
  routeId: string;
  operation: FileOperation;
  paths: string[];
  defaultDestination?: string;
  folderSource?: DataSource;
  pathParam?: string;
}>();

const emit = defineEmits<{
  complete: [];
}>();

const destination = ref(props.defaultDestination ?? "");
const active = ref(false);
const finished = ref(false);
const failed = ref(false);
const statusText = ref("");
const folderNodes = ref<FolderNode[]>([]);
const folderRoot = ref("");
const folderError = ref("");
const folderLoading = ref(false);
const expandedKeys = ref<Record<string, boolean>>({});
const selectedFolderKeys = ref<Record<string, boolean>>({});

const operationLabel = computed(() => {
  switch (props.operation) {
    case FileOperation.Move:
      return "Move";
    case FileOperation.Copy:
      return "Copy";
    default:
      return "File operation";
  }
});

const itemLabel = computed(
  () => `${props.paths.length} item${props.paths.length === 1 ? "" : "s"}`,
);

const headerText = computed(() => `${operationLabel.value} ${itemLabel.value}`);

const destinationLabel = computed(() =>
  props.operation === FileOperation.Move ? "Move to folder" : "Copy to folder",
);

const canBrowseFolders = computed(() => Boolean(props.folderSource?.routeId));

const canStart = computed(
  () =>
    props.paths.length > 0 &&
    !active.value &&
    Boolean(destination.value.trim()),
);

const parentDisabled = computed(
  () =>
    active.value ||
    !canBrowseFolders.value ||
    parentPath(folderRoot.value) === folderRoot.value,
);

function reset(): void {
  active.value = false;
  finished.value = false;
  failed.value = false;
  statusText.value = "";
}

async function runOperation(): Promise<void> {
  if (!canStart.value) return;
  reset();
  active.value = true;
  statusText.value = `${operationLabel.value} started`;
  try {
    await runAction(props.connectionId, props.routeId, props.ctx, {
      paths: props.paths,
      destination: destination.value.trim(),
    });
    active.value = false;
    finished.value = true;
    statusText.value = `${operationLabel.value} complete`;
    emit("complete");
  } catch (e) {
    active.value = false;
    failed.value = true;
    statusText.value =
      e instanceof Error ? e.message : `${operationLabel.value} failed`;
  }
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

      <div class="space-y-2">
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
            :pt="{ wrapper: 'max-h-56 overflow-auto p-1' }"
            @node-select="onFolderSelect"
            @node-expand="onFolderExpand"
          >
            <template #nodeicon>
              <AppIcon
                class="h-4 w-4 shrink-0 text-surface-400"
                :icon="{ type: 'lucide', value: 'folder' }"
                :size="16"
              />
            </template>
          </Tree>
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
            aria-label="Operation destination"
            :disabled="active"
          />
          <span class="block text-xs text-surface-500 dark:text-surface-400">
            Select a folder above or type a destination path. Each item keeps
            its current name.
          </span>
        </label>
      </div>

      <div v-if="active || finished || failed" class="text-sm">
        <span
          class="font-medium"
          :class="
            failed
              ? 'text-danger-600'
              : 'text-surface-800 dark:text-surface-100'
          "
        >
          {{ statusText || operationLabel }}
        </span>
      </div>

      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          outlined
          :disabled="active"
          @click="visible = false"
        />
        <Button
          type="button"
          :disabled="!canStart"
          :loading="active"
          @click="runOperation"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'play' }" :size="15" />
          {{ operationLabel }}
        </Button>
      </div>
    </div>
  </Dialog>
</template>
