<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import FileUpload from "primevue/fileupload";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import ProgressBar from "primevue/progressbar";
import SelectButton from "primevue/selectbutton";
import AppIcon from "../../components/AppIcon.vue";
import type { UploadProgress } from "../../api/dataSource";
import { formatBytes } from "./fileTypes";

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
  statusLabel?: string;
  uploadProgress?: UploadProgress | null;
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

const statusText = computed(() => {
  if (!props.uploadProgress) return props.statusLabel ?? "";
  if (props.uploadProgress.indeterminate)
    return props.statusLabel ?? "Uploading";
  return `${props.statusLabel ?? "Uploading"} · ${formatBytes(
    props.uploadProgress.loaded,
  )} / ${formatBytes(props.uploadProgress.total)}`;
});
</script>

<template>
  <div class="border-b border-surface-200 px-3 py-2 dark:border-surface-800">
    <div class="flex min-h-8 flex-wrap items-center gap-2">
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
          <AppIcon
            :icon="{ type: 'lucide', value: option.icon }"
            :size="15"
            :title="option.label"
          />
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
        title="Upload files"
        :disabled="mutating"
        @uploader="emit('upload', $event)"
      >
        <template #chooseicon>
          <AppIcon :icon="{ type: 'lucide', value: 'upload' }" :size="15" />
        </template>
      </FileUpload>
      <Button
        v-if="canMkdir"
        type="button"
        severity="secondary"
        :disabled="mutating"
        label="New folder"
        title="Create folder"
        @click="emit('mkdir')"
      />
      <Button
        v-if="canShowRename"
        type="button"
        severity="secondary"
        :disabled="!canRename || mutating"
        label="Rename"
        title="Rename selected item"
        @click="emit('rename')"
      />
      <Button
        v-if="canShowDelete"
        type="button"
        severity="danger"
        outlined
        :disabled="!canDelete || mutating"
        label="Delete"
        title="Delete selected item"
        @click="emit('delete')"
      />
      <Button
        v-if="downloadHref"
        as="a"
        severity="secondary"
        :href="downloadHref"
        :download="downloadName"
        label="Download"
        title="Download selected file"
      />
      <Button
        type="button"
        severity="secondary"
        :disabled="loading || mutating"
        title="Refresh folder"
        @click="emit('refresh')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="loading"
        />
        Refresh
      </Button>
    </div>

    <div
      v-if="statusText"
      class="mt-2 flex items-center gap-3 text-xs text-surface-500 dark:text-surface-400"
      role="status"
      aria-live="polite"
    >
      <span class="min-w-0 shrink-0 truncate">{{ statusText }}</span>
      <ProgressBar
        v-if="uploadProgress"
        class="h-1.5 min-w-32 flex-1"
        :mode="uploadProgress.indeterminate ? 'indeterminate' : 'determinate'"
        :value="uploadProgress.percent"
        :show-value="false"
        aria-label="Upload progress"
      />
    </div>
  </div>
</template>
