<script setup lang="ts">
import AppIcon from "../../components/AppIcon.vue";
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
  <div class="h-full overflow-y-auto">
    <p v-if="loading" class="p-3 text-sm text-surface-400">Loading…</p>
    <p v-else-if="error" class="p-3 text-sm text-red-500">{{ error }}</p>
    <p
      v-else-if="!entries.length"
      class="p-6 text-center text-sm text-surface-400"
    >
      This folder is empty.
    </p>
    <ul v-else class="divide-y divide-surface-100 dark:divide-surface-800/70">
      <li v-for="entry in entries" :key="entry.path">
        <button
          type="button"
          class="group flex w-full min-w-0 items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-surface-100 focus-visible:bg-surface-100 focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none focus-visible:ring-inset dark:hover:bg-surface-800 dark:focus-visible:bg-surface-800"
          :class="
            selectedPath === entry.path
              ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-200'
              : ''
          "
          :aria-label="
            entry.isDir ? `Open ${entry.name}` : `Select ${entry.name}`
          "
          @click="entry.isDir ? emit('open', entry) : emit('select', entry)"
          @dblclick="emit('open', entry)"
        >
          <AppIcon
            :icon="{ type: 'name', value: entry.isDir ? 'folder' : 'code' }"
            :size="16"
            class="shrink-0 text-surface-400 group-hover:text-surface-600 dark:group-hover:text-surface-300"
          />
          <span
            class="min-w-0 flex-1 truncate text-surface-700 dark:text-surface-200"
          >
            {{ entry.name }}
          </span>
          <span v-if="!entry.isDir" class="shrink-0 text-xs text-surface-400">
            {{ formatBytes(entry.size) }}
          </span>
          <AppIcon
            v-else
            :icon="{ type: 'name', value: 'chevron-right' }"
            :size="15"
            class="shrink-0 text-surface-300 transition-colors group-hover:text-surface-500 dark:text-surface-600 dark:group-hover:text-surface-300"
          />
        </button>
      </li>
    </ul>
  </div>
</template>
