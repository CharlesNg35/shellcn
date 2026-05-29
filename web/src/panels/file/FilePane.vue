<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";
import type { FileContent, FileEntry } from "../../types/projection";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import FilePreview from "./FilePreview.vue";
import { formatBytes, formatDate } from "./fileTypes";

withDefaults(
  defineProps<{
    selected: FileEntry | null;
    content: FileContent | null;
    canEdit: boolean;
    language: string;
    loading?: boolean;
    error?: string | null;
    mutating?: boolean;
    saving?: boolean;
    dirty?: boolean;
    downloadHref?: string;
  }>(),
  {
    loading: false,
    error: null,
    mutating: false,
    saving: false,
    dirty: false,
    downloadHref: "",
  },
);

const editContent = defineModel<string>("editContent", { default: "" });
const emit = defineEmits<{ save: []; retry: [] }>();
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <header
      v-if="selected && !selected.isDir"
      class="flex items-center gap-3 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'file' }"
        :size="15"
        class="shrink-0 text-surface-400"
      />
      <div class="min-w-0 flex-1">
        <p
          class="truncate text-sm font-medium text-surface-800 dark:text-surface-100"
          :title="selected.path"
        >
          {{ selected.name }}
        </p>
        <p class="truncate text-xs text-surface-400">
          {{ formatBytes(selected.size) }}
          <span v-if="selected.modTime">
            · {{ formatDate(selected.modTime) }}</span
          >
          <span v-if="selected.mode"> · {{ selected.mode }}</span>
          <span v-if="selected.symlink"> · → {{ selected.symlink }}</span>
        </p>
      </div>
      <span v-if="dirty" class="shrink-0 text-xs text-amber-500">Unsaved</span>
      <Button
        v-if="downloadHref"
        as="a"
        severity="secondary"
        size="small"
        :href="downloadHref"
        :download="selected.name"
        title="Download file"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="14" />
        Download
      </Button>
      <Button
        v-if="canEdit"
        type="button"
        size="small"
        label="Save"
        :loading="saving"
        :disabled="!dirty || mutating"
        @click="emit('save')"
      />
    </header>

    <div class="min-h-0 flex-1">
      <CodeTextEditor
        v-if="canEdit"
        v-model:value="editContent"
        :language="language"
        :disabled="mutating"
        :aria-label="selected?.name ? `${selected.name} editor` : 'File editor'"
      />
      <FilePreview
        v-else
        :name="selected?.name ?? ''"
        :content="content"
        :loading="loading"
        :error="error"
        @retry="emit('retry')"
      />
    </div>
  </div>
</template>
