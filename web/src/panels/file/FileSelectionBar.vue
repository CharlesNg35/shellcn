<script setup lang="ts">
import Button from "primevue/button";
import Checkbox from "primevue/checkbox";
import AppIcon from "../../components/AppIcon.vue";

defineProps<{
  count: number;
  allSelected: boolean;
  someSelected: boolean;
  canMove: boolean;
  canCopy: boolean;
  canChmod: boolean;
  canArchive: boolean;
  canDelete: boolean;
  busy: boolean;
}>();

const emit = defineEmits<{
  "toggle-all": [];
  clear: [];
  move: [];
  copy: [];
  chmod: [];
  archive: [];
  delete: [];
}>();
</script>

<template>
  <div
    class="flex flex-wrap items-center gap-2 border-b border-primary-200 bg-primary-50/70 px-3 py-2 dark:border-primary-800 dark:bg-primary-950/40"
    role="region"
    aria-label="Selection actions"
  >
    <span class="flex items-center gap-2 pl-0.5">
      <Checkbox
        :model-value="allSelected"
        :indeterminate="someSelected && !allSelected"
        binary
        aria-label="Select all items"
        @update:model-value="emit('toggle-all')"
      />
    </span>
    <span
      class="text-sm font-medium text-primary-700 dark:text-primary-200"
      aria-live="polite"
    >
      {{ count }} selected
    </span>

    <span
      class="mx-0.5 h-5 w-px bg-primary-200 dark:bg-primary-800"
      aria-hidden="true"
    />

    <Button
      v-if="canArchive"
      type="button"
      severity="secondary"
      class="h-9"
      :disabled="busy"
      title="Download selected as zip"
      @click="emit('archive')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'file-archive' }" :size="15" />
      Download zip
    </Button>
    <Button
      v-if="canMove"
      type="button"
      severity="secondary"
      class="h-9"
      :disabled="busy"
      title="Move selected items"
      @click="emit('move')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'folder-input' }" :size="15" />
      Move
    </Button>
    <Button
      v-if="canCopy"
      type="button"
      severity="secondary"
      class="h-9"
      :disabled="busy"
      title="Copy selected items"
      @click="emit('copy')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'copy' }" :size="15" />
      Copy
    </Button>
    <Button
      v-if="canChmod"
      type="button"
      severity="secondary"
      class="h-9"
      :disabled="busy"
      title="Change permissions of selected items"
      @click="emit('chmod')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'shield' }" :size="15" />
      Permissions
    </Button>
    <Button
      v-if="canDelete"
      type="button"
      severity="danger"
      outlined
      class="h-9"
      :disabled="busy"
      title="Delete selected items"
      @click="emit('delete')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'trash-2' }" :size="15" />
      Delete
    </Button>

    <Button
      type="button"
      severity="secondary"
      text
      class="ml-auto h-9"
      :disabled="busy"
      title="Clear selection"
      @click="emit('clear')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
      Clear
    </Button>
  </div>
</template>
