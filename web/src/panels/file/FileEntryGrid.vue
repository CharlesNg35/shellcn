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
      class="py-12 text-center text-sm text-surface-400"
    >
      {{ emptyText }}
    </p>
    <div
      v-else
      class="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3"
    >
      <div
        v-for="entry in entries"
        :key="entry.path"
        class="relative min-w-0 rounded-lg border border-surface-200 bg-surface-0 p-3 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/50 dark:border-surface-800 dark:bg-surface-950 dark:hover:border-primary-500/60 dark:hover:bg-primary-500/10"
        :class="
          selectedPath === entry.path
            ? 'border-primary-400 bg-primary-50 dark:border-primary-500 dark:bg-primary-500/10'
            : ''
        "
        :aria-current="selectedPath === entry.path || undefined"
        :title="entry.path"
      >
        <span v-if="selectable" class="absolute top-2 right-2" @click.stop>
          <Checkbox
            :model-value="selectedPaths.has(entry.path)"
            binary
            :aria-label="`Select ${entry.name}`"
            @update:model-value="emit('toggle', entry)"
          />
        </span>
        <button
          type="button"
          class="block w-full min-w-0 text-left focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none"
          :aria-label="
            entry.isDir ? `Open ${entry.name}` : `Select ${entry.name}`
          "
          @click="entry.isDir ? emit('open', entry) : emit('select', entry)"
          @dblclick="emit('open', entry)"
        >
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
        </button>
      </div>
    </div>
  </div>
</template>
