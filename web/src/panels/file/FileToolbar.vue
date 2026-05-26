<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import FileUpload from "primevue/fileupload";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import SelectButton from "primevue/selectbutton";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<{
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

const mode = computed({
  get: () => props.viewMode,
  set: (value) => emit("update:viewMode", value as "split" | "grid"),
});

const viewOptions = [
  { label: "Split view", value: "split", icon: "list" },
  { label: "Grid view", value: "grid", icon: "grid" },
];
</script>

<template>
  <div
    class="flex min-h-12 flex-wrap items-center gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
  >
    <SelectButton
      v-model="mode"
      class="mr-2"
      :options="viewOptions"
      option-label="label"
      option-value="value"
      :allow-empty="false"
      aria-label="File browser view"
    >
      <template #option="{ option }">
        <AppIcon :icon="{ type: 'name', value: option.icon }" :size="15" />
        <span class="sr-only">{{ option.label }}</span>
      </template>
    </SelectButton>
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
    >
      <template #chooseicon>
        <AppIcon :icon="{ type: 'name', value: 'upload' }" :size="15" />
      </template>
    </FileUpload>
    <Button
      v-if="canMkdir"
      type="button"
      severity="secondary"
      :disabled="mutating"
      label="New folder"
      @click="emit('mkdir')"
    />
    <Button
      v-if="canShowRename"
      type="button"
      severity="secondary"
      :disabled="!canRename || mutating"
      label="Rename"
      @click="emit('rename')"
    />
    <Button
      v-if="canShowDelete"
      type="button"
      severity="danger"
      outlined
      :disabled="!canDelete || mutating"
      label="Delete"
      @click="emit('delete')"
    />
    <Button
      v-if="downloadHref"
      as="a"
      severity="secondary"
      :href="downloadHref"
      :download="downloadName"
      label="Download"
    />
    <Button
      type="button"
      severity="secondary"
      :disabled="loading"
      label="Refresh"
      @click="emit('refresh')"
    />
  </div>
</template>
