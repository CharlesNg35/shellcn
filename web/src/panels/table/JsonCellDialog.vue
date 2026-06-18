<script setup lang="ts">
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import { dialogRoot } from "@/primevue/preset";
import CodeTextEditor from "../shared/CodeTextEditor.vue";

defineProps<{
  visible: boolean;
  title: string;
  text: string;
  error?: string | null;
  saving?: boolean;
}>();

defineEmits<{
  "update:visible": [value: boolean];
  "update:text": [value: string];
  save: [];
}>();
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="title"
    :dismissable-mask="!saving"
    :closable="!saving"
    :pt="{ root: dialogRoot('max-w-3xl') }"
    @update:visible="$emit('update:visible', $event)"
  >
    <div
      class="h-[52vh] min-h-80 overflow-hidden rounded-md border border-surface-200 dark:border-surface-800"
    >
      <CodeTextEditor
        :value="text"
        language="json"
        aria-label="JSON cell value"
        :disabled="saving"
        @update:value="$emit('update:text', $event)"
      />
    </div>
    <p v-if="error" class="mt-3 text-sm text-red-500">{{ error }}</p>
    <template #footer>
      <Button
        type="button"
        label="Cancel"
        severity="secondary"
        :disabled="saving"
        @click="$emit('update:visible', false)"
      />
      <Button
        type="button"
        label="Save"
        :loading="saving"
        :disabled="saving"
        @click="$emit('save')"
      />
    </template>
  </Dialog>
</template>
