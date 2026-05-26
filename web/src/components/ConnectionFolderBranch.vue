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
} from "./connectionTree";
import type { ConnectionSummary, FolderColor } from "../types/projection";

defineOptions({ name: "ConnectionFolderBranch" });

const props = defineProps<{
  modelValue: ConnectionFolderNode[];
  activeId: string | null;
  expanded: Record<string, boolean>;
  disabled: boolean;
  depth?: number;
}>();

const emit = defineEmits<{
  "update:modelValue": [nodes: ConnectionFolderNode[]];
  "toggle-folder": [folderId: string];
  "menu-action": [action: ConnectionFolderMenuAction];
  "drag-start": [];
  "drag-end": [];
  open: [connection: ConnectionSummary];
}>();

const folders = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const menu = ref<{ toggle: (event: Event) => void } | null>(null);
const menuFolder = ref<ConnectionFolderNode | null>(null);

const colorClasses: Record<
  FolderColor,
  { icon: string; active: string; line: string }
> = {
  slate: {
    icon: "text-slate-500",
    active: "bg-slate-50/80 dark:bg-slate-500/10",
    line: "border-l-slate-300 dark:border-l-slate-700",
  },
  blue: {
    icon: "text-blue-500",
    active: "bg-blue-50/80 dark:bg-blue-500/10",
    line: "border-l-blue-300 dark:border-l-blue-700",
  },
  teal: {
    icon: "text-teal-500",
    active: "bg-teal-50/80 dark:bg-teal-500/10",
    line: "border-l-teal-300 dark:border-l-teal-700",
  },
  emerald: {
    icon: "text-emerald-500",
    active: "bg-emerald-50/80 dark:bg-emerald-500/10",
    line: "border-l-emerald-300 dark:border-l-emerald-700",
  },
  amber: {
    icon: "text-amber-500",
    active: "bg-amber-50/80 dark:bg-amber-500/10",
    line: "border-l-amber-300 dark:border-l-amber-700",
  },
  rose: {
    icon: "text-rose-500",
    active: "bg-rose-50/80 dark:bg-rose-500/10",
    line: "border-l-rose-300 dark:border-l-rose-700",
  },
  violet: {
    icon: "text-violet-500",
    active: "bg-violet-50/80 dark:bg-violet-500/10",
    line: "border-l-violet-300 dark:border-l-violet-700",
  },
  cyan: {
    icon: "text-cyan-500",
    active: "bg-cyan-50/80 dark:bg-cyan-500/10",
    line: "border-l-cyan-300 dark:border-l-cyan-700",
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
  return (
    folder.connections.length +
    folder.children.reduce((sum, child) => sum + totalConnections(child), 0)
  );
}

function folderIconClass(folder: ConnectionFolderNode): string {
  return colorClasses[folder.color]?.icon ?? colorClasses.slate.icon;
}

function folderRowClass(folder: ConnectionFolderNode): string {
  return isExpanded(folder)
    ? (colorClasses[folder.color]?.active ?? colorClasses.slate.active)
    : "";
}

function branchLineClass(folder: ConnectionFolderNode): string {
  return colorClasses[folder.color]?.line ?? colorClasses.slate.line;
}
</script>

<template>
  <div class="min-w-0">
    <VueDraggable
      v-model="folders"
      group="folders"
      handle=".folder-drag-handle"
      :disabled="disabled"
      :animation="150"
      ghost-class="opacity-40"
      class="min-h-3 space-y-1"
      @start="emit('drag-start')"
      @end="emit('drag-end')"
    >
      <section v-for="folder in folders" :key="folder.id" class="min-w-0">
        <div
          class="group flex min-h-9 items-center gap-1.5 rounded-md px-1.5 text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
          :class="folderRowClass(folder)"
        >
          <Button
            text
            rounded
            severity="secondary"
            size="small"
            :aria-label="
              isExpanded(folder) ? 'Collapse folder' : 'Expand folder'
            "
            class="h-7 w-7 p-0"
            @click="emit('toggle-folder', folder.id)"
          >
            <AppIcon
              :icon="{
                type: 'name',
                value: isExpanded(folder) ? 'chevron-down' : 'chevron-right',
              }"
              :size="14"
            />
          </Button>

          <span
            class="folder-drag-handle inline-flex h-7 w-7 cursor-grab touch-none items-center justify-center rounded active:cursor-grabbing"
            :class="folderIconClass(folder)"
            title="Drag folder"
            aria-label="Drag folder"
          >
            <AppIcon
              :icon="{
                type: 'name',
                value: isExpanded(folder) ? 'folder-open' : 'folder',
              }"
              :size="17"
            />
          </span>

          <button
            type="button"
            class="min-w-0 flex-1 truncate text-left font-medium text-surface-800 dark:text-surface-100"
            @click="emit('toggle-folder', folder.id)"
          >
            {{ folder.name }}
          </button>

          <span class="min-w-5 text-right text-xs text-surface-400">
            {{ totalConnections(folder) }}
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
            :aria-controls="`folder-menu-${folder.id}`"
            @click.stop="toggleMenu($event, folder)"
          >
            <AppIcon
              :icon="{ type: 'name', value: 'ellipsis-vertical' }"
              :size="15"
            />
          </Button>
        </div>

        <div
          v-show="isExpanded(folder)"
          class="mt-1 ml-4 border-l pl-2"
          :class="branchLineClass(folder)"
        >
          <VueDraggable
            v-model="folder.connections"
            group="connections"
            handle=".connection-drag-handle"
            :disabled="disabled"
            :animation="150"
            ghost-class="opacity-40"
            class="min-h-3 space-y-1"
            @start="emit('drag-start')"
            @end="emit('drag-end')"
          >
            <ConnectionSidebarItem
              v-for="connection in folder.connections"
              :key="connection.id"
              :connection="connection"
              :active="activeId === connection.id"
              @open="emit('open', $event)"
            />
          </VueDraggable>

          <ConnectionFolderBranch
            v-show="isExpanded(folder)"
            v-model="folder.children"
            :active-id="activeId"
            :expanded="expanded"
            :disabled="disabled"
            :depth="(depth ?? 0) + 1"
            class="mt-1"
            @toggle-folder="emit('toggle-folder', $event)"
            @menu-action="emit('menu-action', $event)"
            @drag-start="emit('drag-start')"
            @drag-end="emit('drag-end')"
            @open="emit('open', $event)"
          />
        </div>
      </section>
    </VueDraggable>

    <Menu
      :id="menuFolder ? `folder-menu-${menuFolder.id}` : 'folder-menu'"
      ref="menu"
      :model="menuItems"
      popup
    />
  </div>
</template>
