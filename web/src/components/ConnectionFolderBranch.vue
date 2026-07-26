<script setup lang="ts">
import { computed, ref } from "vue";
import { VueDraggable } from "vue-draggable-plus";
import Button from "primevue/button";
import Menu from "primevue/menu";
import AppIcon from "./AppIcon.vue";
import ConnectionSidebarItem from "./ConnectionSidebarItem.vue";
import type {
  ConnectionFolderMenuAction,
  ConnectionFolderNode,
  ConnectionTreeItem,
} from "./connectionTree";
import type { ConnectionSummary, FolderColor } from "../types/projection";

defineOptions({ name: "ConnectionFolderBranch" });

// A branch mounts one component per row, so a long list is trimmed and search
// becomes the way to reach the rest. Reordering is off while trimmed: Sortable
// would emit only the rendered slice and silently drop everything past the cut.
const MAX_RENDERED_ITEMS = 200;

const props = defineProps<{
  modelValue: ConnectionTreeItem[];
  activeId: string | null;
  expanded: Record<string, boolean>;
  disabled: boolean;
  dragging?: boolean;
  droppedId?: string;
  collapsed?: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [items: ConnectionTreeItem[]];
  "toggle-folder": [folderId: string];
  "reveal-folder": [folderId: string];
  "menu-action": [action: ConnectionFolderMenuAction];
  "drag-start": [];
  "drag-end": [droppedId?: string];
  open: [connection: ConnectionSummary];
}>();

const items = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const trimmed = computed(() => props.modelValue.length > MAX_RENDERED_ITEMS);
const visibleItems = computed(() =>
  trimmed.value ? props.modelValue.slice(0, MAX_RENDERED_ITEMS) : items.value,
);
const hiddenCount = computed(
  () => props.modelValue.length - visibleItems.value.length,
);
const reorderDisabled = computed(
  () => props.disabled || Boolean(props.collapsed) || trimmed.value,
);

const menu = ref<{ toggle: (event: Event) => void } | null>(null);
const menuFolder = ref<ConnectionFolderNode | null>(null);

const colorClasses: Record<FolderColor, { icon: string }> = {
  slate: { icon: "text-slate-500" },
  blue: { icon: "text-blue-500" },
  teal: { icon: "text-teal-500" },
  emerald: { icon: "text-emerald-500" },
  amber: { icon: "text-amber-500" },
  rose: { icon: "text-rose-500" },
  violet: { icon: "text-violet-500" },
  cyan: { icon: "text-cyan-500" },
};

const menuItems = computed(() => [
  {
    label: "Rename",
    command: () => emitMenuAction("rename"),
  },
  { separator: true },
  {
    label: "Delete",
    command: () => emitMenuAction("delete"),
  },
]);

// Folders live only at the root level: a folder may never be dropped inside
// another folder, so the tree stays exactly two deep (folder → connections).
function onMove(evt: { dragged?: HTMLElement; to?: HTMLElement }): boolean {
  if (reorderDisabled.value) return false; // never reorder a filtered, rail or trimmed view
  const draggingFolder = evt.dragged?.hasAttribute("data-folder-id") ?? false;
  const intoFolder = Boolean(evt.to?.closest("[data-folder-id]"));
  return !(draggingFolder && intoFolder);
}

function emitMenuAction(action: ConnectionFolderMenuAction["key"]): void {
  if (!menuFolder.value) return;
  emit("menu-action", { key: action, folder: menuFolder.value });
}

function toggleMenu(event: MouseEvent, folder: ConnectionFolderNode): void {
  menuFolder.value = folder;
  menu.value?.toggle(event);
}

function isExpanded(folder: ConnectionFolderNode): boolean {
  return props.expanded[folder.id] ?? false;
}

function totalConnections(folder: ConnectionFolderNode): number {
  return folder.children.reduce((sum, item) => {
    if (item.kind === "connection") return sum + 1;
    return sum + totalConnections(item);
  }, 0);
}

function folderIconClass(folder: ConnectionFolderNode): string {
  return colorClasses[folder.color]?.icon ?? colorClasses.slate.icon;
}

// The id of whatever was dropped, read from Sortable's dragged element so the
// sidebar can briefly highlight where it landed.
function onEnd(event: unknown): void {
  const item = (event as { item?: HTMLElement } | null | undefined)?.item;
  const el =
    item instanceof HTMLElement
      ? (item.closest<HTMLElement>("[data-connection-id], [data-folder-id]") ??
        item)
      : item;
  emit("drag-end", el?.dataset.connectionId ?? el?.dataset.folderId);
}
</script>

<template>
  <div class="min-w-0">
    <VueDraggable
      v-model="items"
      group="sidebar-items"
      handle=".connection-sidebar-drag-item"
      :disabled="reorderDisabled"
      :on-move="onMove"
      :animation="150"
      chosen-class="connection-sidebar-sortable-chosen"
      drag-class="connection-sidebar-sortable-drag"
      ghost-class="connection-sidebar-sortable-ghost"
      class="min-h-3 space-y-1"
      @start="emit('drag-start')"
      @end="onEnd"
    >
      <template
        v-for="item in visibleItems"
        :key="item.kind === 'folder' ? item.id : item.connection.id"
      >
        <ConnectionSidebarItem
          v-if="item.kind === 'connection'"
          :connection="item.connection"
          :active="activeId === item.connection.id"
          :dragging="dragging"
          :highlighted="item.connection.id === droppedId"
          :collapsed="collapsed"
          @open="emit('open', $event)"
        />

        <section v-else class="min-w-0" :data-folder-id="item.id">
          <div
            v-if="collapsed"
            class="connection-sidebar-drag-item mx-auto flex h-10 w-10 items-center justify-center rounded-md"
            :data-folder-row-id="item.id"
          >
            <button
              type="button"
              class="relative flex h-9 w-9 items-center justify-center rounded-full transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
              :class="folderIconClass(item)"
              v-tooltip.right="
                `${item.name} · ${totalConnections(item)} connections`
              "
              :aria-label="`Open folder ${item.name}`"
              @click="emit('reveal-folder', item.id)"
            >
              <AppIcon :icon="{ type: 'lucide', value: 'folder' }" :size="16" />
              <span
                v-if="totalConnections(item)"
                class="absolute -right-0.5 -bottom-0.5 min-w-4 rounded-full bg-surface-200 px-1 text-center text-[10px] leading-4 font-medium text-surface-600 dark:bg-surface-700 dark:text-surface-200"
                aria-hidden="true"
              >
                {{ totalConnections(item) }}
              </span>
            </button>
          </div>

          <div
            v-else
            class="connection-sidebar-drag-item group mx-1 flex min-h-10 w-[calc(100%-0.5rem)] items-center gap-2.5 overflow-hidden rounded-md px-2 py-1.5 text-sm transition-colors"
            :data-folder-row-id="item.id"
            :class="[
              !dragging && 'hover:bg-surface-100 dark:hover:bg-surface-800',
              item.id === droppedId && 'bg-surface-100 dark:bg-surface-800',
            ]"
          >
            <span
              class="shrink-0 rounded p-0.5"
              :class="folderIconClass(item)"
              aria-hidden="true"
            >
              <AppIcon
                :icon="{
                  type: 'lucide',
                  value: isExpanded(item) ? 'folder-open' : 'folder',
                }"
                :size="16"
              />
            </span>

            <button
              type="button"
              class="flex min-w-0 flex-1 flex-col overflow-hidden text-left font-medium text-surface-800 dark:text-surface-100"
              :title="item.name"
              :aria-label="`${isExpanded(item) ? 'Collapse' : 'Expand'} ${item.name}`"
              @click="emit('toggle-folder', item.id)"
            >
              <span class="block max-w-full truncate" :title="item.name">
                {{ item.name }}
              </span>
              <span class="block max-w-full truncate text-xs text-surface-400">
                Folder
              </span>
            </button>

            <span class="min-w-5 shrink-0 text-right text-xs text-surface-400">
              {{ totalConnections(item) }}
            </span>

            <Button
              text
              rounded
              severity="secondary"
              size="small"
              class="m-0 -mr-2.25 h-7 w-7 shrink-0 justify-center p-0 opacity-70 transition-opacity group-hover:opacity-100"
              title="Folder actions"
              aria-label="Folder actions"
              aria-haspopup="true"
              :aria-controls="`folder-menu-${item.id}`"
              @click.stop="toggleMenu($event, item)"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'ellipsis-vertical' }"
                :size="15"
              />
            </Button>
          </div>

          <ConnectionFolderBranch
            v-if="!collapsed && isExpanded(item)"
            v-model="item.children"
            :active-id="activeId"
            :expanded="expanded"
            :disabled="disabled"
            :dragging="dragging"
            :dropped-id="droppedId"
            :collapsed="collapsed"
            class="mt-1 pl-4"
            @toggle-folder="emit('toggle-folder', $event)"
            @reveal-folder="emit('reveal-folder', $event)"
            @menu-action="emit('menu-action', $event)"
            @drag-start="emit('drag-start')"
            @drag-end="emit('drag-end', $event)"
            @open="emit('open', $event)"
          />
        </section>
      </template>
    </VueDraggable>

    <p
      v-if="hiddenCount > 0 && !collapsed"
      class="px-3 py-1.5 text-xs text-surface-400"
    >
      {{ hiddenCount }} more — search to narrow
    </p>

    <Menu
      :id="menuFolder ? `folder-menu-${menuFolder.id}` : 'folder-menu'"
      ref="menu"
      :model="menuItems"
      popup
    />
  </div>
</template>
