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
import ConnectionFolderDialog from "./ConnectionFolderDialog.vue";
import type {
  ConnectionFolder,
  ConnectionSummary,
  FolderColor,
} from "../types/projection";

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
const folderConnections = ref<Record<string, ConnectionSummary[]>>({});
const expanded = useStorage<Record<string, boolean>>(
  "shellcn:connection-folders:expanded",
  {},
  localStorage,
  { mergeDefaults: true },
);
const showFolderDialog = ref(false);
const editingFolder = ref<ConnectionFolder | null>(null);
const savingLayout = ref(false);

const colorClasses: Record<
  FolderColor,
  { icon: string; accent: string; active: string }
> = {
  slate: {
    icon: "text-slate-500",
    accent: "border-l-slate-500",
    active:
      "bg-slate-50 text-slate-800 dark:bg-slate-500/10 dark:text-slate-200",
  },
  blue: {
    icon: "text-blue-500",
    accent: "border-l-blue-500",
    active: "bg-blue-50 text-blue-800 dark:bg-blue-500/10 dark:text-blue-200",
  },
  teal: {
    icon: "text-teal-500",
    accent: "border-l-teal-500",
    active: "bg-teal-50 text-teal-800 dark:bg-teal-500/10 dark:text-teal-200",
  },
  emerald: {
    icon: "text-emerald-500",
    accent: "border-l-emerald-500",
    active:
      "bg-emerald-50 text-emerald-800 dark:bg-emerald-500/10 dark:text-emerald-200",
  },
  amber: {
    icon: "text-amber-500",
    accent: "border-l-amber-500",
    active:
      "bg-amber-50 text-amber-800 dark:bg-amber-500/10 dark:text-amber-200",
  },
  rose: {
    icon: "text-rose-500",
    accent: "border-l-rose-500",
    active: "bg-rose-50 text-rose-800 dark:bg-rose-500/10 dark:text-rose-200",
  },
  violet: {
    icon: "text-violet-500",
    accent: "border-l-violet-500",
    active:
      "bg-violet-50 text-violet-800 dark:bg-violet-500/10 dark:text-violet-200",
  },
  cyan: {
    icon: "text-cyan-500",
    accent: "border-l-cyan-500",
    active: "bg-cyan-50 text-cyan-800 dark:bg-cyan-500/10 dark:text-cyan-200",
  },
};

const sortedFolders = computed(() =>
  [...conns.folders].sort(
    (a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name),
  ),
);

const visibleFolders = computed(() =>
  props.query.trim()
    ? sortedFolders.value.filter(
        (folder) => folderConnections.value[folder.id]?.length,
      )
    : sortedFolders.value,
);

const emptyFiltered = computed(
  () =>
    conns.loaded &&
    Boolean(props.query.trim()) &&
    !rootConnections.value.length &&
    !visibleFolders.value.length,
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
    if (conn?.folderId)
      expanded.value = { ...expanded.value, [conn.folderId]: true };
  },
  { immediate: true },
);

function rebuildLists(): void {
  const q = props.query.trim().toLowerCase();
  const folderIds = new Set(conns.folders.map((f) => f.id));
  const lists: Record<string, ConnectionSummary[]> = Object.fromEntries(
    conns.folders.map((f) => [f.id, []]),
  );
  const root: ConnectionSummary[] = [];
  for (const c of conns.connections) {
    if (q && !`${c.name} ${c.protocol}`.toLowerCase().includes(q)) continue;
    if (c.folderId && folderIds.has(c.folderId)) lists[c.folderId].push(c);
    else root.push(c);
  }
  const sortConnections = (a: ConnectionSummary, b: ConnectionSummary) =>
    (a.sortOrder ?? Number.MAX_SAFE_INTEGER) -
      (b.sortOrder ?? Number.MAX_SAFE_INTEGER) || a.name.localeCompare(b.name);
  rootConnections.value = root.sort(sortConnections);
  for (const id of Object.keys(lists))
    lists[id] = lists[id].sort(sortConnections);
  folderConnections.value = lists;
}

function folderClass(folder: ConnectionFolder): string {
  return colorClasses[folder.color]?.active ?? colorClasses.slate.active;
}

function folderIconClass(folder: ConnectionFolder): string {
  return colorClasses[folder.color]?.icon ?? colorClasses.slate.icon;
}

function folderAccentClass(folder: ConnectionFolder): string {
  return colorClasses[folder.color]?.accent ?? colorClasses.slate.accent;
}

function dotClass(c: ConnectionSummary): string {
  if (c.status === "offline") return "bg-red-500";
  if (ws.isConnected(c.id)) return "bg-emerald-400";
  return "bg-surface-300 dark:bg-surface-600";
}

function dotTitle(c: ConnectionSummary): string {
  if (c.status === "offline") return "Agent offline";
  if (ws.isConnected(c.id)) return "Connected";
  return "Idle";
}

function shareTitle(c: ConnectionSummary): string {
  if (c.sharedWithMe) return `Shared with you · ${c.access ?? "use"}`;
  if (c.sharedByMe) return "Shared by you";
  return "";
}

function go(c: ConnectionSummary): void {
  ws.open(c.id);
  void router.push({ name: "connection", params: { id: c.id } });
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
    message: `Delete “${folder.name}”? Connections inside it will move to the main list.`,
    acceptLabel: "Delete",
    accept: async () => {
      await conns.deleteFolder(folder.id);
      notify.success("Folder deleted", folder.name);
      rebuildLists();
    },
  });
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
    rootConnections.value.forEach((c, index) =>
      items.push({ connectionId: c.id, sortOrder: index }),
    );
    for (const folder of conns.folders) {
      (folderConnections.value[folder.id] ?? []).forEach((c, index) =>
        items.push({
          connectionId: c.id,
          folderId: folder.id,
          sortOrder: index,
        }),
      );
    }
    await conns.saveLayout(items);
    rebuildLists();
  } catch (e) {
    notify.error("Could not save sidebar order", (e as Error).message);
    await conns.refresh();
  } finally {
    savingLayout.value = false;
  }
}

function onDragEnd(): void {
  if (props.query.trim()) return;
  void nextTick(() => persistLayout());
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
          @click="openNewFolder"
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
        @end="onDragEnd"
      >
        <button
          v-for="c in rootConnections"
          :key="c.id"
          type="button"
          class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="
            activeId === c.id
              ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
              : ''
          "
          @click="go(c)"
        >
          <span
            class="connection-drag-handle cursor-grab text-surface-300 opacity-0 transition-opacity group-hover:opacity-100"
            aria-hidden="true"
          >
            <AppIcon
              :icon="{ type: 'name', value: 'grip-vertical' }"
              :size="14"
            />
          </span>
          <AppIcon :icon="c.icon" :size="16" class="text-surface-500" />
          <span class="flex min-w-0 flex-1 flex-col">
            <span class="truncate text-surface-800 dark:text-surface-100">{{
              c.name
            }}</span>
            <span class="truncate text-xs text-surface-400">{{
              c.protocol
            }}</span>
          </span>
          <span
            class="h-2 w-2 shrink-0 rounded-full"
            :class="dotClass(c)"
            :title="dotTitle(c)"
          />
          <AppIcon
            v-if="c.sharedWithMe || c.sharedByMe"
            :icon="{
              type: 'name',
              value: c.sharedWithMe ? 'users' : 'share-2',
            }"
            :size="14"
            class="shrink-0 text-surface-400"
            :title="shareTitle(c)"
          />
        </button>
      </VueDraggable>

      <section
        v-for="folder in visibleFolders"
        :key="folder.id"
        class="mt-2 rounded-md border-l-2"
        :class="folderAccentClass(folder)"
      >
        <div
          class="group flex items-center gap-1 rounded-md px-1 py-1"
          :class="folderClass(folder)"
        >
          <Button
            text
            rounded
            severity="secondary"
            size="small"
            :aria-label="
              expanded[folder.id] ? 'Collapse folder' : 'Expand folder'
            "
            @click="toggleFolder(folder.id)"
          >
            <AppIcon
              :icon="{
                type: 'name',
                value: expanded[folder.id] ? 'chevron-down' : 'chevron-right',
              }"
              :size="14"
            />
          </Button>
          <AppIcon
            :icon="{
              type: 'name',
              value: expanded[folder.id] ? 'folder-open' : 'folder',
            }"
            :size="16"
            :class="folderIconClass(folder)"
          />
          <button
            type="button"
            class="min-w-0 flex-1 truncate text-left text-sm font-medium"
            @click="toggleFolder(folder.id)"
          >
            {{ folder.name }}
          </button>
          <span class="text-xs text-surface-400">
            {{ folderConnections[folder.id]?.length ?? 0 }}
          </span>
          <Button
            text
            rounded
            severity="secondary"
            size="small"
            class="opacity-0 group-hover:opacity-100"
            title="Edit folder"
            aria-label="Edit folder"
            @click="editFolder(folder)"
          >
            <AppIcon :icon="{ type: 'name', value: 'pencil' }" :size="14" />
          </Button>
          <Button
            text
            rounded
            severity="danger"
            size="small"
            class="opacity-0 group-hover:opacity-100"
            title="Delete folder"
            aria-label="Delete folder"
            @click="askDeleteFolder(folder)"
          >
            <AppIcon :icon="{ type: 'name', value: 'trash' }" :size="14" />
          </Button>
        </div>

        <VueDraggable
          v-show="expanded[folder.id]"
          v-model="folderConnections[folder.id]"
          group="connections"
          handle=".connection-drag-handle"
          :disabled="Boolean(query.trim())"
          :animation="150"
          ghost-class="opacity-40"
          class="min-h-3 space-y-1 pt-1 pl-3"
          @end="onDragEnd"
        >
          <button
            v-for="c in folderConnections[folder.id]"
            :key="c.id"
            type="button"
            class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
            :class="
              activeId === c.id
                ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
                : ''
            "
            @click="go(c)"
          >
            <span
              class="connection-drag-handle cursor-grab text-surface-300 opacity-0 transition-opacity group-hover:opacity-100"
              aria-hidden="true"
            >
              <AppIcon
                :icon="{ type: 'name', value: 'grip-vertical' }"
                :size="14"
              />
            </span>
            <AppIcon :icon="c.icon" :size="16" class="text-surface-500" />
            <span class="flex min-w-0 flex-1 flex-col">
              <span class="truncate text-surface-800 dark:text-surface-100">{{
                c.name
              }}</span>
              <span class="truncate text-xs text-surface-400">{{
                c.protocol
              }}</span>
            </span>
            <span
              class="h-2 w-2 shrink-0 rounded-full"
              :class="dotClass(c)"
              :title="dotTitle(c)"
            />
            <AppIcon
              v-if="c.sharedWithMe || c.sharedByMe"
              :icon="{
                type: 'name',
                value: c.sharedWithMe ? 'users' : 'share-2',
              }"
              :size="14"
              class="shrink-0 text-surface-400"
              :title="shareTitle(c)"
            />
          </button>
        </VueDraggable>
        <p
          v-if="expanded[folder.id] && !folderConnections[folder.id]?.length"
          class="px-5 py-2 text-xs text-surface-400"
        >
          Empty folder
        </p>
      </section>

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
      @saved="rebuildLists"
    />
  </div>
</template>
