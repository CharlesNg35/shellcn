<script setup lang="ts">
import AppIcon from "../../components/AppIcon.vue";
import PanelError from "../shared/PanelError.vue";
import type { FileEntry } from "../../types/projection";
import { formatBytes } from "./fileTypes";

defineProps<{
  entries: FileEntry[];
  selectedPath?: string;
  loading: boolean;
  error?: string | null;
}>();
const emit = defineEmits<{
  select: [entry: FileEntry];
  open: [entry: FileEntry];
}>();
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <p v-if="loading" class="text-sm text-surface-400">Loading…</p>
    <PanelError v-else-if="error" :message="error" />
    <p
      v-else-if="!entries.length"
      class="py-12 text-center text-sm text-surface-400"
    >
      This folder is empty.
    </p>
    <div
      v-else
      class="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3"
    >
      <button
        v-for="entry in entries"
        :key="entry.path"
        type="button"
        class="min-w-0 rounded-lg border border-surface-200 bg-surface-0 p-3 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/50 dark:border-surface-800 dark:bg-surface-950 dark:hover:border-primary-500/60 dark:hover:bg-primary-500/10"
        :class="
          selectedPath === entry.path
            ? 'border-primary-400 bg-primary-50 dark:border-primary-500 dark:bg-primary-500/10'
            : ''
        "
        @click="emit('select', entry)"
        @dblclick="emit('open', entry)"
      >
        <AppIcon
          :icon="{ type: 'name', value: entry.isDir ? 'folder' : 'code' }"
          :size="28"
          class="mb-2 text-surface-400"
        />
        <span
          class="block truncate text-sm font-medium text-surface-800 dark:text-surface-100"
        >
          {{ entry.name }}
        </span>
        <span class="mt-1 block truncate text-xs text-surface-400">
          {{ entry.isDir ? "Folder" : formatBytes(entry.size) }}
        </span>
      </button>
    </div>
  </div>
</template>
