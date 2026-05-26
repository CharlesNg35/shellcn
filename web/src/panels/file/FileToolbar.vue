<script setup lang="ts">
import Button from "primevue/button";
import FileUpload from "primevue/fileupload";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import AppIcon from "../../components/AppIcon.vue";

defineProps<{
  viewMode: "split" | "grid";
  canUpload: boolean;
  canMkdir: boolean;
  canRename: boolean;
  canDelete: boolean;
  canShowRename: boolean;
  canShowDelete: boolean;
  downloadHref: string;
  downloadName?: string;
  multipleUpload: boolean;
  maxUploadBytes?: number;
  uploadFieldName: string;
  mutating: boolean;
  loading: boolean;
}>();

const emit = defineEmits<{
  "update:viewMode": [mode: "split" | "grid"];
  upload: [event: FileUploadUploaderEvent];
  mkdir: [];
  rename: [];
  delete: [];
  refresh: [];
}>();
</script>

<template>
  <div
    class="flex min-h-12 flex-wrap items-center gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
  >
    <div
      class="mr-2 inline-flex rounded-md border border-surface-300 p-0.5 dark:border-surface-700"
      aria-label="File browser view"
    >
      <button
        type="button"
        class="rounded px-2 py-1 text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800"
        :class="
          viewMode === 'split'
            ? 'bg-surface-100 text-surface-900 dark:bg-surface-800 dark:text-surface-0'
            : ''
        "
        title="Split view"
        @click="emit('update:viewMode', 'split')"
      >
        <AppIcon :icon="{ type: 'name', value: 'list' }" :size="15" />
      </button>
      <button
        type="button"
        class="rounded px-2 py-1 text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800"
        :class="
          viewMode === 'grid'
            ? 'bg-surface-100 text-surface-900 dark:bg-surface-800 dark:text-surface-0'
            : ''
        "
        title="Grid view"
        @click="emit('update:viewMode', 'grid')"
      >
        <AppIcon :icon="{ type: 'name', value: 'grid' }" :size="15" />
      </button>
    </div>
    <FileUpload
      v-if="canUpload"
      mode="basic"
      :name="uploadFieldName"
      :multiple="multipleUpload"
      :max-file-size="maxUploadBytes"
      custom-upload
      auto
      choose-label="Upload"
      :disabled="mutating"
      @uploader="emit('upload', $event)"
    />
    <Button
      v-if="canMkdir"
      type="button"
      :disabled="mutating"
      label="New folder"
      @click="emit('mkdir')"
    />
    <Button
      v-if="canShowRename"
      type="button"
      :disabled="!canRename || mutating"
      label="Rename"
      @click="emit('rename')"
    />
    <Button
      v-if="canShowDelete"
      type="button"
      :disabled="!canDelete || mutating"
      label="Delete"
      @click="emit('delete')"
    />
    <Button
      v-if="downloadHref"
      as="a"
      :href="downloadHref"
      :download="downloadName"
      label="Download"
    />
    <Button
      type="button"
      :disabled="loading"
      label="Refresh"
      @click="emit('refresh')"
    />
  </div>
</template>
