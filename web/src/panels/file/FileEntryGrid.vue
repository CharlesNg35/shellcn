<script setup lang="ts">
import Checkbox from "primevue/checkbox";
import AppIcon from "@/components/AppIcon.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import PanelError from "../shared/PanelError.vue";
import type { FileEntry } from "@/types/projection";
import { formatBytes, formatDate, iconFor } from "./fileTypes";

withDefaults(
  defineProps<{
    entries: FileEntry[];
    selectedPath?: string;
    loading: boolean;
    error?: string | null;
    emptyText?: string;
    selectable?: boolean;
    selectedPaths?: Set<string>;
  }>(),
  {
    selectedPath: undefined,
    error: null,
    emptyText: "This folder is empty.",
    selectable: false,
    selectedPaths: () => new Set<string>(),
  },
);
const emit = defineEmits<{
  select: [entry: FileEntry];
  open: [entry: FileEntry];
  retry: [];
  toggle: [entry: FileEntry];
}>();

// Roving focus: arrow keys move between grid cells (wrapping by column count
// derived from the rendered layout) so the grid is keyboard-navigable.
function cells(grid: HTMLElement): HTMLElement[] {
  return [...grid.querySelectorAll<HTMLElement>('[role="option"]')];
}

function columnCount(items: HTMLElement[]): number {
  if (items.length < 2) return items.length || 1;
  const top = items[0]!.offsetTop;
  let cols = 0;
  for (const item of items) {
    if (item.offsetTop !== top) break;
    cols += 1;
  }
  return cols || 1;
}

function moveFocus(event: KeyboardEvent, delta: number): void {
  const cell = event.currentTarget as HTMLElement;
  const grid = cell.closest<HTMLElement>('[role="listbox"]');
  if (!grid) return;
  const items = cells(grid);
  const index = items.indexOf(cell);
  const next = items[index + delta];
  if (next) {
    event.preventDefault();
    next.focus();
  }
}

function moveRow(event: KeyboardEvent, dir: 1 | -1): void {
  const cell = event.currentTarget as HTMLElement;
  const grid = cell.closest<HTMLElement>('[role="listbox"]');
  if (!grid) return;
  const items = cells(grid);
  moveFocus(event, dir * columnCount(items));
}

function activate(entry: FileEntry): void {
  if (entry.isDir) emit("open", entry);
  else emit("select", entry);
}
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <SkeletonList v-if="loading" />
    <PanelError
      v-else-if="error"
      :message="error"
      retryable
      @retry="emit('retry')"
    />
    <p
      v-else-if="!entries.length"
      role="status"
      aria-live="polite"
      class="py-12 text-center text-sm text-surface-400"
    >
      {{ emptyText }}
    </p>
    <div
      v-else
      role="listbox"
      aria-label="Files"
      class="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3"
    >
      <div
        v-for="(entry, i) in entries"
        :key="entry.path"
        role="option"
        :aria-selected="selectedPath === entry.path"
        :tabindex="
          selectedPath === entry.path || (!selectedPath && i === 0) ? 0 : -1
        "
        class="relative min-w-0 cursor-pointer rounded-lg border border-surface-200 bg-surface-0 p-3 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/50 focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none dark:border-surface-800 dark:bg-surface-950 dark:hover:border-primary-500/60 dark:hover:bg-primary-500/10"
        :class="
          selectedPath === entry.path
            ? 'border-primary-400 bg-primary-50 dark:border-primary-500 dark:bg-primary-500/10'
            : ''
        "
        :aria-label="
          entry.isDir ? `Open ${entry.name}` : `Select ${entry.name}`
        "
        :title="entry.path"
        @click="activate(entry)"
        @dblclick="emit('open', entry)"
        @keydown.enter.prevent="activate(entry)"
        @keydown.space.prevent="activate(entry)"
        @keydown.right="moveFocus($event, 1)"
        @keydown.left="moveFocus($event, -1)"
        @keydown.down="moveRow($event, 1)"
        @keydown.up="moveRow($event, -1)"
      >
        <span v-if="selectable" class="absolute top-2 right-2" @click.stop>
          <Checkbox
            :model-value="selectedPaths.has(entry.path)"
            binary
            :aria-label="`Select ${entry.name}`"
            @update:model-value="emit('toggle', entry)"
          />
        </span>
        <AppIcon
          :icon="{ type: 'lucide', value: iconFor(entry.name, entry.isDir) }"
          :size="28"
          class="mb-2"
          :class="
            entry.isDir
              ? 'text-amber-500 dark:text-amber-400'
              : 'text-surface-400'
          "
        />
        <span
          class="block truncate text-sm font-medium text-surface-800 dark:text-surface-100"
          :title="entry.name"
        >
          {{ entry.name }}
        </span>
        <span class="mt-1 block truncate text-xs text-surface-400">
          {{ entry.isDir ? "Folder" : formatBytes(entry.size) }}
        </span>
        <span
          v-if="entry.modTime"
          class="mt-0.5 block truncate text-xs text-surface-400/80 tabular-nums"
        >
          {{ formatDate(entry.modTime) }}
        </span>
      </div>
    </div>
  </div>
</template>
