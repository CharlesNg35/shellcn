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
  "drag-add": [preference?: ConnectionTreeDropPreference];
  "drag-end": [preference?: ConnectionTreeDropPreference];
  open: [connection: ConnectionSummary];
}>();

const items = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

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

function dragAdd(event: unknown): void {
  const sortable = (event ?? {}) as {
    item?: HTMLElement;
  };
  emit("drag-add", {
    connectionId: sortable.item?.dataset.connectionId,
    folderId: sortable.item?.dataset.folderId,
    targetParentId: props.parentId,
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
      @add="dragAdd"
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
            class="group flex min-h-10 w-full items-center gap-2.5 overflow-hidden rounded-md px-2 py-1.5 text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
          >
            <span
              class="folder-drag-handle shrink-0 cursor-grab touch-none rounded p-0.5 active:cursor-grabbing"
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
              class="flex min-w-0 flex-1 flex-col overflow-hidden text-left font-medium text-surface-800 dark:text-surface-100"
              :title="item.name"
              :aria-label="`${isExpanded(item) ? 'Collapse' : 'Expand'} ${item.name}`"
              @click="emit('toggle-folder', item.id)"
            >
              <span class="block max-w-full truncate" :title="item.name">{{
                item.name
              }}</span>
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
              class="m-0 h-7 w-7 shrink-0 justify-center p-0 opacity-70 transition-opacity group-hover:opacity-100"
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
            @drag-add="emit('drag-add', $event)"
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
