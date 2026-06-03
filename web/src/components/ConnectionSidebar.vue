<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useEventListener, useStorage, useTimeoutFn } from "@vueuse/core";
import Button from "primevue/button";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import { useConfirmAction } from "../composables/useConfirmAction";
import AppIcon from "./AppIcon.vue";
import ConnectionFolderBranch from "./ConnectionFolderBranch.vue";
import ConnectionFolderDialog from "./ConnectionFolderDialog.vue";
import type {
  ConnectionFolderMenuAction,
  ConnectionFolderNode,
  ConnectionNode,
  ConnectionTreeItem,
} from "./connectionTree";
import type { ConnectionFolder, ConnectionSummary } from "../types/projection";

const props = defineProps<{
  activeId: string | null;
  query: string;
}>();

const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const auth = useAuthStore();
const router = useRouter();
const notify = useNotify();
const { confirmDanger } = useConfirmAction();

const rootItems = ref<ConnectionTreeItem[]>([]);
const dragging = ref(false);
const settling = ref(false);
const hoverSuppressed = computed(() => dragging.value || settling.value);
const droppedId = ref<string | undefined>();
const treeRenderKey = ref(0);
const scrollEl = ref<HTMLElement | null>(null);
const listScrolled = ref(false);

const { start: startDropFade, stop: stopDropFade } = useTimeoutFn(
  () => {
    droppedId.value = undefined;
  },
  1500,
  { immediate: false },
);

useEventListener(scrollEl, "pointermove", () => {
  if (settling.value) settling.value = false;
  if (droppedId.value) {
    droppedId.value = undefined;
    stopDropFade();
  }
});
const expanded = useStorage<Record<string, boolean>>(
  "shellcn:connection-folders:expanded",
  {},
  localStorage,
  { mergeDefaults: true },
);
const activeOnly = ref(false);

const filtering = computed(
  () => Boolean(props.query.trim()) || activeOnly.value,
);
const displayExpanded = computed<Record<string, boolean>>(() => {
  if (!filtering.value) return expanded.value;
  return Object.fromEntries(conns.folders.map((f) => [f.id, true]));
});
const showFolderDialog = ref(false);
const editingFolder = ref<ConnectionFolder | null>(null);
const savingLayout = ref(false);
let pendingPersist = false;

const emptyFiltered = computed(
  () =>
    conns.loaded &&
    conns.connections.length > 0 &&
    (Boolean(props.query.trim()) || activeOnly.value) &&
    !rootItems.value.length,
);

watch(
  () =>
    [
      conns.connections,
      conns.folders,
      props.query,
      activeOnly.value,
      ws.connected,
    ] as const,
  rebuildLists,
  { immediate: true, deep: true },
);

watch(
  () => [rootItems.value, conns.loaded, props.query] as const,
  () => void nextTick(updateScrollShadow),
  { deep: true },
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
  const sortItems = (a: ConnectionTreeItem, b: ConnectionTreeItem) =>
    itemSortOrder(a) - itemSortOrder(b) ||
    itemLabel(a).localeCompare(itemLabel(b));

  const folderIds = new Set(conns.folders.map((f) => f.id));
  const nodeById = new Map<string, ConnectionFolderNode>();
  for (const folder of conns.folders) {
    nodeById.set(folder.id, {
      ...folder,
      kind: "folder",
      children: [],
    });
  }

  const roots: ConnectionTreeItem[] = [];
  for (const connection of conns.connections) {
    if (
      q &&
      !`${connection.name} ${connection.protocol}`.toLowerCase().includes(q)
    ) {
      continue;
    }
    if (activeOnly.value && !ws.isConnected(connection.id)) continue;
    const item: ConnectionNode = { kind: "connection", connection };
    if (connection.folderId && folderIds.has(connection.folderId)) {
      nodeById.get(connection.folderId)?.children.push(item);
    } else {
      roots.push(item);
    }
  }

  for (const node of nodeById.values()) {
    roots.push(node);
  }

  const sortTree = (items: ConnectionTreeItem[]): ConnectionTreeItem[] => {
    items.sort(sortItems);
    for (const item of items) {
      if (item.kind === "folder") sortTree(item.children);
    }
    return items;
  };

  const tree = sortTree(roots);
  rootItems.value = q || activeOnly.value ? filterTree(tree) : tree;
}

function itemSortOrder(item: ConnectionTreeItem): number {
  if (item.kind === "folder") return item.sortOrder;
  return item.connection.sortOrder ?? Number.MAX_SAFE_INTEGER;
}

function itemLabel(item: ConnectionTreeItem): string {
  return item.kind === "folder" ? item.name : item.connection.name;
}

function filterTree(items: ConnectionTreeItem[]): ConnectionTreeItem[] {
  const out: ConnectionTreeItem[] = [];
  for (const item of items) {
    if (item.kind === "connection") {
      out.push(item);
      continue;
    }
    const children = filterTree(item.children);
    if (children.length) out.push({ ...item, children });
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

function openNewFolder(): void {
  editingFolder.value = null;
  showFolderDialog.value = true;
}

function editFolder(folder: ConnectionFolder): void {
  editingFolder.value = folder;
  showFolderDialog.value = true;
}

function askDeleteFolder(folder: ConnectionFolder): void {
  confirmDanger({
    header: "Delete folder",
    message: `Delete "${folder.name}"? Its connections will move to the main list.`,
    acceptLabel: "Delete",
    accept: async () => {
      await conns.deleteFolder(folder.id);
      notify.success("Folder deleted", folder.name);
      rebuildLists();
    },
  });
}

function handleFolderMenu(action: ConnectionFolderMenuAction): void {
  if (action.key === "rename") {
    editFolder(action.folder);
  } else if (action.key === "delete") {
    askDeleteFolder(action.folder);
  }
}

function toggleFolder(id: string): void {
  expanded.value = { ...expanded.value, [id]: !expanded.value[id] };
}

function remountTree(): void {
  treeRenderKey.value += 1;
}

async function persistLayout(): Promise<void> {
  if (savingLayout.value) {
    pendingPersist = true;
    return;
  }
  savingLayout.value = true;
  try {
    pendingPersist = false;
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

    collectLayout(rootItems.value, undefined, items, folders);

    await conns.saveLayout(items, folders);
  } catch (e) {
    notify.error("Could not save sidebar order", (e as Error).message);
    await conns.refresh();
    rebuildLists();
    remountTree();
  } finally {
    savingLayout.value = false;
    if (pendingPersist) void persistLayout();
  }
}

function applyOptimisticLayout(): void {
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

  collectLayout(rootItems.value, undefined, items, folders);

  const itemById = new Map(items.map((item) => [item.connectionId, item]));
  conns.connections = conns.connections.map((connection) => {
    const item = itemById.get(connection.id);
    return item
      ? {
          ...connection,
          folderId: item.folderId,
          sortOrder: item.sortOrder,
        }
      : connection;
  });

  const folderById = new Map(
    folders.map((folder) => [folder.folderId, folder]),
  );
  conns.folders = conns.folders.map((folder) => {
    const item = folderById.get(folder.id);
    return item
      ? {
          ...folder,
          parentId: item.parentId,
          sortOrder: item.sortOrder,
        }
      : folder;
  });
}

function collectLayout(
  nodes: ConnectionTreeItem[],
  parentId: string | undefined,
  items: Array<{ connectionId: string; folderId?: string; sortOrder: number }>,
  folders: Array<{ folderId: string; parentId?: string; sortOrder: number }>,
): void {
  nodes.forEach((item, index) => {
    if (item.kind === "connection") {
      items.push({
        connectionId: item.connection.id,
        folderId: parentId,
        sortOrder: index,
      });
      return;
    }
    folders.push({ folderId: item.id, parentId: undefined, sortOrder: index });
    collectLayout(item.children, item.id, items, folders);
  });
}

function onDragStart(): void {
  dragging.value = true;
  settling.value = false;
}

function onDragEnd(dropped?: string): void {
  dragging.value = false;
  if (filtering.value) return;
  settling.value = true;
  droppedId.value = dropped;
  if (dropped) startDropFade();
  applyOptimisticLayout();
  remountTree();
  void nextTick(updateScrollShadow);
  void persistLayout();
}

function updateScrollShadow(): void {
  listScrolled.value = (scrollEl.value?.scrollTop ?? 0) > 2;
}

onMounted(updateScrollShadow);

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
          :title="
            activeOnly ? 'Showing active only' : 'Show active connections only'
          "
          :aria-label="
            activeOnly ? 'Show all connections' : 'Show active connections only'
          "
          :aria-pressed="activeOnly"
          @click="activeOnly = !activeOnly"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'activity' }"
            :size="15"
            :class="activeOnly ? 'text-primary-500' : ''"
          />
        </Button>
        <Button
          v-if="auth.canCreate"
          text
          rounded
          severity="secondary"
          size="small"
          title="New folder"
          aria-label="New folder"
          @click="openNewFolder()"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'folder-plus' }"
            :size="15"
          />
        </Button>
        <slot name="create" />
      </div>
    </div>

    <div class="relative min-h-0 flex-1">
      <div
        data-sidebar-scroll-shadow
        class="pointer-events-none absolute inset-x-0 top-0 z-10 h-3 bg-linear-to-b from-surface-950/5 to-transparent transition-opacity duration-150 dark:from-black/20"
        :class="listScrolled ? 'opacity-100' : 'opacity-0'"
        aria-hidden="true"
      />
      <div
        ref="scrollEl"
        data-sidebar-scroll-region
        class="connection-sidebar-list h-full overflow-y-auto py-1"
        :class="{ 'connection-sidebar-list--dragging': hoverSuppressed }"
        @scroll="updateScrollShadow"
      >
        <ConnectionFolderBranch
          :key="`${treeRenderKey}:${filtering}`"
          v-model="rootItems"
          :active-id="activeId"
          :expanded="displayExpanded"
          :disabled="filtering"
          :dragging="hoverSuppressed"
          :dropped-id="droppedId"
          @toggle-folder="toggleFolder"
          @menu-action="handleFolderMenu"
          @drag-start="onDragStart"
          @drag-end="onDragEnd"
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
          {{
            query.trim()
              ? `No connections match "${query}".`
              : "No active connections."
          }}
        </p>
        <div
          v-else-if="conns.loaded && !conns.connections.length"
          class="flex flex-col items-center gap-1.5 px-4 py-10 text-center"
        >
          <span
            class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-surface-100 text-surface-400 dark:bg-surface-800"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'server' }" :size="18" />
          </span>
          <p class="text-sm font-medium text-surface-600 dark:text-surface-300">
            No connections yet
          </p>
          <p class="text-xs text-surface-400">
            Use the + above to add your first one.
          </p>
        </div>
      </div>
    </div>

    <ConnectionFolderDialog
      v-model:visible="showFolderDialog"
      :folder="editingFolder"
      @saved="rebuildLists"
    />
  </div>
</template>

<style scoped>
.connection-sidebar-list--dragging :deep(.connection-sidebar-drag-item:hover),
.connection-sidebar-list :deep(.connection-sidebar-sortable-chosen),
.connection-sidebar-list :deep(.connection-sidebar-sortable-drag) {
  background-color: transparent;
}

.connection-sidebar-list :deep(.connection-sidebar-sortable-ghost) {
  background-color: transparent;
  opacity: 0.4;
}
</style>
