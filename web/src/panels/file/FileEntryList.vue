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

// Roving focus: arrow keys move between rows so the list is keyboard-navigable.
function moveFocus(event: KeyboardEvent, dir: 1 | -1): void {
  const li = (event.currentTarget as HTMLElement).closest("li");
  const sibling =
    dir === 1 ? li?.nextElementSibling : li?.previousElementSibling;
  const button = sibling?.querySelector("button");
  if (button) {
    event.preventDefault();
    button.focus();
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto">
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
      class="p-6 text-center text-sm text-surface-400"
    >
      {{ emptyText }}
    </p>
    <ul
      v-else
      role="listbox"
      aria-label="Files"
      class="divide-y divide-surface-100 dark:divide-surface-800/70"
    >
      <li
        v-for="entry in entries"
        :key="entry.path"
        role="option"
        :aria-selected="selectedPath === entry.path"
        class="group flex items-center transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
        :class="
          selectedPath === entry.path
            ? 'bg-primary-50 dark:bg-primary-500/10'
            : ''
        "
      >
        <span
          v-if="selectable"
          class="flex shrink-0 items-center pl-3"
          @click.stop
        >
          <Checkbox
            :model-value="selectedPaths.has(entry.path)"
            binary
            :aria-label="`Select ${entry.name}`"
            @update:model-value="emit('toggle', entry)"
          />
        </span>
        <button
          type="button"
          class="flex w-full min-w-0 items-center gap-2 px-3 py-2 text-left text-sm transition-colors focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none focus-visible:ring-inset"
          :class="
            selectedPath === entry.path
              ? 'text-primary-700 dark:text-primary-200'
              : ''
          "
          :aria-label="
            entry.isDir ? `Open ${entry.name}` : `Select ${entry.name}`
          "
          :title="entry.path"
          @click="entry.isDir ? emit('open', entry) : emit('select', entry)"
          @dblclick="emit('open', entry)"
          @keydown.down="moveFocus($event, 1)"
          @keydown.up="moveFocus($event, -1)"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: iconFor(entry.name, entry.isDir) }"
            :size="16"
            class="shrink-0"
            :class="
              entry.isDir
                ? 'text-amber-500 dark:text-amber-400'
                : 'text-surface-400 group-hover:text-surface-600 dark:group-hover:text-surface-300'
            "
          />
          <span
            class="min-w-0 flex-1 truncate text-surface-700 dark:text-surface-200"
            :title="entry.name"
          >
            {{ entry.name }}
          </span>
          <span
            v-if="entry.modTime"
            class="shrink-0 text-xs whitespace-nowrap text-surface-400 tabular-nums"
          >
            {{ formatDate(entry.modTime) }}
          </span>
          <span
            v-if="!entry.isDir"
            class="shrink-0 text-xs whitespace-nowrap text-surface-400 tabular-nums"
          >
            {{ formatBytes(entry.size) }}
          </span>
          <AppIcon
            v-else
            :icon="{ type: 'lucide', value: 'chevron-right' }"
            :size="15"
            class="shrink-0 text-surface-300 transition-colors group-hover:text-surface-500 dark:text-surface-600 dark:group-hover:text-surface-300"
          />
        </button>
      </li>
    </ul>
  </div>
</template>
