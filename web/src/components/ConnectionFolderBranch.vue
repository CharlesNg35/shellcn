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
  ConnectionTreeDropPreference,
  ConnectionTreeItem,
} from "./connectionTree";
import type { ConnectionSummary, FolderColor } from "../types/projection";

defineOptions({ name: "ConnectionFolderBranch" });

const props = defineProps<{
  modelValue: ConnectionTreeItem[];
  activeId: string | null;
  expanded: Record<string, boolean>;
  disabled: boolean;
  parentId?: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [items: ConnectionTreeItem[]];
  "toggle-folder": [folderId: string];
  "menu-action": [action: ConnectionFolderMenuAction];
  "drag-start": [];
  "drag-end": [preference?: ConnectionTreeDropPreference];
  open: [connection: ConnectionSummary];
}>();

const items = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const menu = ref<{ toggle: (event: Event) => void } | null>(null);
const menuFolder = ref<ConnectionFolderNode | null>(null);

const colorClasses: Record<FolderColor, { icon: string; active: string }> = {
  slate: {
    icon: "text-slate-500",
    active: "bg-slate-50/80 dark:bg-slate-500/10",
  },
  blue: {
    icon: "text-blue-500",
    active: "bg-blue-50/80 dark:bg-blue-500/10",
  },
  teal: {
    icon: "text-teal-500",
    active: "bg-teal-50/80 dark:bg-teal-500/10",
  },
  emerald: {
    icon: "text-emerald-500",
    active: "bg-emerald-50/80 dark:bg-emerald-500/10",
  },
  amber: {
    icon: "text-amber-500",
    active: "bg-amber-50/80 dark:bg-amber-500/10",
  },
  rose: {
    icon: "text-rose-500",
    active: "bg-rose-50/80 dark:bg-rose-500/10",
  },
  violet: {
    icon: "text-violet-500",
    active: "bg-violet-50/80 dark:bg-violet-500/10",
  },
  cyan: {
    icon: "text-cyan-500",
    active: "bg-cyan-50/80 dark:bg-cyan-500/10",
  },
};

const menuItems = computed(() => [
  {
    label: "New subfolder",
    command: () => emitMenuAction("new-child"),
  },
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

function folderRowClass(folder: ConnectionFolderNode): string {
  return isExpanded(folder)
    ? (colorClasses[folder.color]?.active ?? colorClasses.slate.active)
    : "";
}

function dragEnd(event: unknown): void {
  const sortable = (event ?? {}) as {
    item?: HTMLElement;
    to?: HTMLElement;
  };
  emit("drag-end", {
    connectionId: sortable.item?.dataset.connectionId,
    folderId: sortable.item?.dataset.folderId,
    targetParentId: sortable.to?.dataset.parentFolderId || undefined,
  });
}
</script>

<template>
  <div class="min-w-0">
    <VueDraggable
      v-model="items"
      group="sidebar-items"
      handle=".connection-drag-handle, .folder-drag-handle"
      :data-parent-folder-id="parentId ?? ''"
      :disabled="disabled"
      :animation="150"
      ghost-class="opacity-40"
      class="min-h-3 space-y-1"
      @start="emit('drag-start')"
      @end="dragEnd"
    >
      <template
        v-for="item in items"
        :key="item.kind === 'folder' ? item.id : item.connection.id"
      >
        <ConnectionSidebarItem
          v-if="item.kind === 'connection'"
          :connection="item.connection"
          :active="activeId === item.connection.id"
          @open="emit('open', $event)"
        />

        <section v-else class="min-w-0" :data-folder-id="item.id">
          <div
            class="group flex min-h-10 w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
            :class="folderRowClass(item)"
          >
            <span
              class="folder-drag-handle cursor-grab touch-none rounded p-0.5 active:cursor-grabbing"
              :class="folderIconClass(item)"
              title="Drag folder"
              aria-label="Drag folder"
            >
              <AppIcon
                :icon="{
                  type: 'name',
                  value: isExpanded(item) ? 'folder-open' : 'folder',
                }"
                :size="16"
              />
            </span>

            <button
              type="button"
              class="flex min-w-0 flex-1 flex-col text-left font-medium text-surface-800 dark:text-surface-100"
              @click="emit('toggle-folder', item.id)"
            >
              <span class="truncate">{{ item.name }}</span>
              <span class="truncate text-xs text-surface-400"> Folder </span>
            </button>

            <span class="min-w-5 text-right text-xs text-surface-400">
              {{ totalConnections(item) }}
            </span>

            <Button
              text
              rounded
              severity="secondary"
              size="small"
              class="h-7 w-7 p-0 opacity-70 transition-opacity group-hover:opacity-100"
              title="Folder actions"
              aria-label="Folder actions"
              aria-haspopup="true"
              :aria-controls="`folder-menu-${item.id}`"
              @click.stop="toggleMenu($event, item)"
            >
              <AppIcon
                :icon="{ type: 'name', value: 'ellipsis-vertical' }"
                :size="15"
              />
            </Button>
          </div>

          <ConnectionFolderBranch
            v-show="isExpanded(item)"
            v-model="item.children"
            :active-id="activeId"
            :expanded="expanded"
            :disabled="disabled"
            :parent-id="item.id"
            class="mt-1 pl-4"
            @toggle-folder="emit('toggle-folder', $event)"
            @menu-action="emit('menu-action', $event)"
            @drag-start="emit('drag-start')"
            @drag-end="emit('drag-end', $event)"
            @open="emit('open', $event)"
          />
        </section>
      </template>
    </VueDraggable>

    <Menu
      :id="menuFolder ? `folder-menu-${menuFolder.id}` : 'folder-menu'"
      ref="menu"
      :model="menuItems"
      popup
    />
  </div>
</template>
