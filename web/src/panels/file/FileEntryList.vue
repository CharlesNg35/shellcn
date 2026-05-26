<script setup lang="ts">
import Button from "primevue/button";
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
    <ul v-else>
      <li v-for="entry in entries" :key="entry.path" class="flex items-stretch">
        <button
          type="button"
          class="flex min-w-0 flex-1 items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-800"
          :class="
            selectedPath === entry.path
              ? 'bg-surface-100 dark:bg-surface-800'
              : ''
          "
          @click="emit('select', entry)"
          @dblclick="emit('open', entry)"
        >
          <AppIcon
            :icon="{ type: 'name', value: entry.isDir ? 'folder' : 'code' }"
            :size="15"
            class="shrink-0 text-surface-400"
          />
          <span class="flex-1 truncate text-surface-700 dark:text-surface-200">
            {{ entry.name }}
          </span>
          <span v-if="!entry.isDir" class="text-xs text-surface-400">
            {{ formatBytes(entry.size) }}
          </span>
        </button>
        <Button
          v-if="entry.isDir"
          type="button"
          :aria-label="`Open ${entry.name}`"
          :pt="{
            root: 'w-8 shrink-0 rounded-none px-0 text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800',
          }"
          @click="emit('open', entry)"
        >
          <AppIcon :icon="{ type: 'name', value: 'chevron-right' }" />
        </Button>
      </li>
    </ul>
  </div>
</template>
