<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useStorage } from "@vueuse/core";
import { VueDraggable } from "vue-draggable-plus";
import Button from "primevue/button";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useNotify } from "../composables/useNotify";
import { useConfirmAction } from "../composables/useConfirmAction";
import AppIcon from "./AppIcon.vue";
import ConnectionFolderBranch from "./ConnectionFolderBranch.vue";
import ConnectionFolderDialog from "./ConnectionFolderDialog.vue";
import ConnectionSidebarItem from "./ConnectionSidebarItem.vue";
import type {
  ConnectionFolderMenuAction,
  ConnectionFolderNode,
} from "./connectionTree";
import type { ConnectionFolder, ConnectionSummary } from "../types/projection";

const props = defineProps<{
  activeId: string | null;
  query: string;
}>();

const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const router = useRouter();
const notify = useNotify();
const { confirmDanger } = useConfirmAction();

const rootConnections = ref<ConnectionSummary[]>([]);
const folderTree = ref<ConnectionFolderNode[]>([]);
const dragging = ref(false);
const expanded = useStorage<Record<string, boolean>>(
  "shellcn:connection-folders:expanded",
  {},
  localStorage,
  { mergeDefaults: true },
);
const showFolderDialog = ref(false);
const editingFolder = ref<ConnectionFolder | null>(null);
const newFolderParentId = ref<string | null>(null);
const savingLayout = ref(false);

const emptyFiltered = computed(
  () =>
    conns.loaded &&
    Boolean(props.query.trim()) &&
    !rootConnections.value.length &&
    !folderTree.value.length,
);

watch(
  () => [conns.connections, conns.folders, props.query] as const,
  rebuildLists,
  { immediate: true, deep: true },
);

watch(
  () => props.activeId,
  (id) => {
    if (!id) return;
    const conn = conns.byId(id);
    if (!conn?.folderId) return;
    openFolderAncestors(conn.folderId);
  },
  { immediate: true },
);

function rebuildLists(): void {
  const q = props.query.trim().toLowerCase();
  const sortConnections = (a: ConnectionSummary, b: ConnectionSummary) =>
    (a.sortOrder ?? Number.MAX_SAFE_INTEGER) -
      (b.sortOrder ?? Number.MAX_SAFE_INTEGER) || a.name.localeCompare(b.name);
  const sortFolders = (a: ConnectionFolderNode, b: ConnectionFolderNode) =>
    a.sortOrder - b.sortOrder || a.name.localeCompare(b.name);

  const folderIds = new Set(conns.folders.map((f) => f.id));
  const nodeById = new Map<string, ConnectionFolderNode>();
  for (const folder of conns.folders) {
    nodeById.set(folder.id, {
      ...folder,
      children: [],
      connections: [],
    });
  }

  const root: ConnectionSummary[] = [];
  for (const connection of conns.connections) {
    if (
      q &&
      !`${connection.name} ${connection.protocol}`.toLowerCase().includes(q)
    ) {
      continue;
    }
    if (connection.folderId && folderIds.has(connection.folderId)) {
      nodeById.get(connection.folderId)?.connections.push(connection);
    } else {
      root.push(connection);
    }
  }

  for (const node of nodeById.values()) {
    node.connections.sort(sortConnections);
  }

  const roots: ConnectionFolderNode[] = [];
  for (const node of nodeById.values()) {
    if (node.parentId && nodeById.has(node.parentId)) {
      nodeById.get(node.parentId)?.children.push(node);
    } else {
      roots.push(node);
    }
  }

  const sortTree = (nodes: ConnectionFolderNode[]): ConnectionFolderNode[] => {
    nodes.sort(sortFolders);
    for (const node of nodes) sortTree(node.children);
    return nodes;
  };

  const tree = sortTree(roots);
  rootConnections.value = root.sort(sortConnections);
  folderTree.value = q ? filterTree(tree) : tree;
}

function filterTree(nodes: ConnectionFolderNode[]): ConnectionFolderNode[] {
  const out: ConnectionFolderNode[] = [];
  for (const node of nodes) {
    const children = filterTree(node.children);
    if (node.connections.length || children.length) {
      out.push({ ...node, children });
      if (children.length || node.connections.length) {
        expanded.value = { ...expanded.value, [node.id]: true };
      }
    }
  }
  return out;
}

function openFolderAncestors(folderId: string): void {
  const byID = new Map(conns.folders.map((folder) => [folder.id, folder]));
  const next = { ...expanded.value };
  let current = byID.get(folderId);
  while (current) {
    next[current.id] = true;
    current = current.parentId ? byID.get(current.parentId) : undefined;
  }
  expanded.value = next;
}

function openNewFolder(parentId?: string): void {
  editingFolder.value = null;
  newFolderParentId.value = parentId ?? null;
  showFolderDialog.value = true;
}

function editFolder(folder: ConnectionFolder): void {
  editingFolder.value = folder;
  newFolderParentId.value = null;
  showFolderDialog.value = true;
}

function askDeleteFolder(folder: ConnectionFolder): void {
  confirmDanger({
    header: "Delete folder",
    message: folder.parentId
      ? `Delete "${folder.name}"? Its connections and subfolders will move up one level.`
      : `Delete "${folder.name}"? Its connections and subfolders will move to the main list.`,
    acceptLabel: "Delete",
    accept: async () => {
      await conns.deleteFolder(folder.id);
      notify.success("Folder deleted", folder.name);
      rebuildLists();
    },
  });
}

function handleFolderMenu(action: ConnectionFolderMenuAction): void {
  if (action.key === "new-child") {
    expanded.value = { ...expanded.value, [action.folder.id]: true };
    openNewFolder(action.folder.id);
  } else if (action.key === "rename") {
    editFolder(action.folder);
  } else if (action.key === "delete") {
    askDeleteFolder(action.folder);
  }
}

function toggleFolder(id: string): void {
  expanded.value = { ...expanded.value, [id]: !expanded.value[id] };
}

async function persistLayout(): Promise<void> {
  if (savingLayout.value) return;
  savingLayout.value = true;
  try {
    const items: Array<{
      connectionId: string;
      folderId?: string;
      sortOrder: number;
    }> = [];
    const folders: Array<{
      folderId: string;
      parentId?: string;
      sortOrder: number;
    }> = [];

    rootConnections.value.forEach((connection, index) =>
      items.push({ connectionId: connection.id, sortOrder: index }),
    );
    collectLayout(folderTree.value, undefined, items, folders);

    await conns.saveLayout(items, folders);
    rebuildLists();
  } catch (e) {
    notify.error("Could not save sidebar order", (e as Error).message);
    await conns.refresh();
  } finally {
    savingLayout.value = false;
  }
}

function collectLayout(
  nodes: ConnectionFolderNode[],
  parentId: string | undefined,
  items: Array<{ connectionId: string; folderId?: string; sortOrder: number }>,
  folders: Array<{ folderId: string; parentId?: string; sortOrder: number }>,
): void {
  nodes.forEach((folder, index) => {
    folders.push({ folderId: folder.id, parentId, sortOrder: index });
    folder.connections.forEach((connection, connectionIndex) =>
      items.push({
        connectionId: connection.id,
        folderId: folder.id,
        sortOrder: connectionIndex,
      }),
    );
    collectLayout(folder.children, folder.id, items, folders);
  });
}

function onDragEnd(): void {
  if (props.query.trim()) return;
  void nextTick(() => persistLayout());
}

function onDragStart(): void {
  dragging.value = true;
}

function afterDragEnd(): void {
  onDragEnd();
  window.setTimeout(() => {
    dragging.value = false;
  }, 0);
}

function go(connection: ConnectionSummary): void {
  if (dragging.value) return;
  ws.open(connection.id);
  void router.push({ name: "connection", params: { id: connection.id } });
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div class="flex items-center justify-between px-2 pt-3 pb-1">
      <p class="text-xs font-medium tracking-wide text-surface-400 uppercase">
        Connections
      </p>
      <div class="flex items-center gap-0.5">
        <Button
          text
          rounded
          severity="secondary"
          size="small"
          title="New folder"
          aria-label="New folder"
          @click="openNewFolder()"
        >
          <AppIcon :icon="{ type: 'name', value: 'folder-plus' }" :size="15" />
        </Button>
        <slot name="create" />
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto">
      <VueDraggable
        v-model="rootConnections"
        group="connections"
        handle=".connection-drag-handle"
        :disabled="Boolean(query.trim())"
        :animation="150"
        ghost-class="opacity-40"
        class="min-h-3 space-y-1"
        @start="onDragStart"
        @end="afterDragEnd"
      >
        <ConnectionSidebarItem
          v-for="connection in rootConnections"
          :key="connection.id"
          :connection="connection"
          :active="activeId === connection.id"
          @open="go"
        />
      </VueDraggable>

      <ConnectionFolderBranch
        v-model="folderTree"
        :active-id="activeId"
        :expanded="expanded"
        :disabled="Boolean(query.trim())"
        class="mt-2"
        @toggle-folder="toggleFolder"
        @menu-action="handleFolderMenu"
        @drag-start="onDragStart"
        @drag-end="afterDragEnd"
        @open="go"
      />

      <div v-if="!conns.loaded" class="space-y-1.5 px-1 pt-1">
        <div
          v-for="n in 5"
          :key="n"
          class="h-9 animate-pulse rounded-md bg-surface-200/60 dark:bg-surface-800/60"
        />
      </div>
      <p
        v-else-if="emptyFiltered"
        class="px-2 py-6 text-center text-sm text-surface-400"
      >
        No connections match "{{ query }}".
      </p>
      <div
        v-else-if="conns.loaded && !conns.connections.length"
        class="flex flex-col items-center gap-1.5 px-4 py-10 text-center"
      >
        <span
          class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-surface-100 text-surface-400 dark:bg-surface-800"
        >
          <AppIcon :icon="{ type: 'name', value: 'server' }" :size="18" />
        </span>
        <p class="text-sm font-medium text-surface-600 dark:text-surface-300">
          No connections yet
        </p>
        <p class="text-xs text-surface-400">
          Use the + above to add your first one.
        </p>
      </div>
    </div>

    <ConnectionFolderDialog
      v-model:visible="showFolderDialog"
      :folder="editingFolder"
      :parent-id="newFolderParentId"
      @saved="rebuildLists"
    />
  </div>
</template>
