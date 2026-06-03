<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import FileUpload from "primevue/fileupload";
import type { FileUploadUploaderEvent } from "primevue/fileupload";
import ProgressBar from "primevue/progressbar";
import Select from "primevue/select";
import SelectButton from "primevue/selectbutton";
import AppIcon from "../../components/AppIcon.vue";
import type { UploadProgress } from "../../api/dataSource";
import type { FileSortKey } from "./fileTypes";
import { formatBytes } from "./fileTypes";
import { inputClass } from "../../primevue/preset";
import { cn } from "../../utils/cn";

const filter = defineModel<string>("filter", { default: "" });
const sortKey = defineModel<FileSortKey>("sortKey", { default: "name" });
const sortDir = defineModel<"asc" | "desc">("sortDir", { default: "asc" });

const sortOptions = [
  { label: "Name", value: "name" },
  { label: "Size", value: "size" },
  { label: "Modified", value: "modified" },
];

const searchClass = cn(inputClass, "h-9 pl-8 pr-8");

function toggleSortDir(): void {
  sortDir.value = sortDir.value === "asc" ? "desc" : "asc";
}

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
  { label: "Split view", value: "split", icon: "panel-left" },
  { label: "Grid view", value: "grid", icon: "layout-grid" },
];

// The selected-item action group (and its divider) only appears when at least
// one of those actions is available for the current selection.
const hasContextActions = computed(
  () =>
    props.canShowRename || props.canShowDelete || Boolean(props.downloadHref),
);

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
      <FileUpload
        v-if="canUpload"
        mode="basic"
        class="[&_button]:h-9"
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
        class="h-9"
        :disabled="mutating"
        title="Create folder"
        @click="emit('mkdir')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'folder-plus' }" :size="15" />
        New folder
      </Button>

      <span
        v-if="hasContextActions"
        class="mx-0.5 h-5 w-px bg-surface-200 dark:bg-surface-800"
        aria-hidden="true"
      />
      <Button
        v-if="canShowRename"
        type="button"
        severity="secondary"
        class="h-9 w-9 px-0!"
        :disabled="!canRename || mutating"
        title="Rename selected item"
        aria-label="Rename selected item"
        @click="emit('rename')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="15" />
      </Button>
      <Button
        v-if="downloadHref"
        as="a"
        severity="secondary"
        class="h-9 w-9 px-0!"
        :href="downloadHref"
        :download="downloadName"
        title="Download selected file"
        aria-label="Download selected file"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="15" />
      </Button>
      <Button
        v-if="canShowDelete"
        type="button"
        severity="danger"
        outlined
        class="h-9 w-9 px-0!"
        :disabled="!canDelete || mutating"
        title="Delete selected item"
        aria-label="Delete selected item"
        @click="emit('delete')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'trash-2' }" :size="15" />
      </Button>

      <div class="ml-auto flex flex-wrap items-center gap-2">
        <div class="relative w-44 sm:w-56">
          <AppIcon
            :icon="{ type: 'lucide', value: 'search' }"
            :size="15"
            class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-surface-400"
          />
          <input
            v-model="filter"
            type="search"
            placeholder="Filter files…"
            aria-label="Filter files"
            :class="searchClass"
          />
          <button
            v-if="filter"
            type="button"
            aria-label="Clear filter"
            title="Clear filter"
            class="absolute top-1/2 right-1.5 flex -translate-y-1/2 items-center rounded p-1 text-surface-400 transition-colors hover:text-surface-700 focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none dark:hover:text-surface-200"
            @click="filter = ''"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
          </button>
        </div>

        <div class="flex items-center gap-1">
          <div class="w-32">
            <Select
              v-model="sortKey"
              class="h-9"
              :options="sortOptions"
              option-label="label"
              option-value="value"
              aria-label="Sort files by"
            />
          </div>
          <Button
            type="button"
            severity="secondary"
            class="h-9 w-9 px-0!"
            :title="sortDir === 'asc' ? 'Ascending' : 'Descending'"
            :aria-label="`Sort direction: ${sortDir === 'asc' ? 'ascending' : 'descending'}`"
            @click="toggleSortDir"
          >
            <AppIcon
              :icon="{
                type: 'lucide',
                value: sortDir === 'asc' ? 'arrow-up' : 'arrow-down',
              }"
              :size="15"
            />
          </Button>
        </div>

        <Button
          type="button"
          severity="secondary"
          class="h-9 w-9 px-0!"
          title="Refresh folder"
          aria-label="Refresh folder"
          :disabled="loading || mutating"
          @click="emit('refresh')"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'refresh-cw' }"
            :size="15"
            :loading="loading"
          />
        </Button>

        <SelectButton
          v-model="mode"
          class="h-9 p-px!"
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
      </div>
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
