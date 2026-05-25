<script setup lang="ts">
import Dialog from "primevue/dialog";
import Button from "primevue/button";

withDefaults(
  defineProps<{
    visible: boolean;
    title: string;
    message: string;
    confirmLabel?: string;
    danger?: boolean;
    busy?: boolean;
  }>(),
  { confirmLabel: "Confirm", danger: false, busy: false },
);
const emit = defineEmits<{
  "update:visible": [value: boolean];
  confirm: [];
}>();
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="title"
    :closable="!busy"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="text-sm text-surface-600 dark:text-surface-300">{{ message }}</p>
    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{
            root: 'rounded-md px-3 py-1.5 text-sm text-surface-600 hover:bg-surface-100 dark:text-surface-300 dark:hover:bg-surface-800',
          }"
          @click="emit('update:visible', false)"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :disabled="busy"
          :pt="{
            root: danger
              ? 'rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50'
              : 'rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50',
          }"
          @click="emit('confirm')"
        >
          {{ busy ? "Working…" : confirmLabel }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>
