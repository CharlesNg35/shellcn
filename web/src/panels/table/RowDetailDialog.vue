<script setup lang="ts">
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import { dialogRoot } from "../../primevue/preset";

export interface DetailItem {
  key: string;
  label: string;
  text: string;
  // Presentational badge class when the source column is a badge; the dialog
  // stays dumb and renders whatever the caller computed.
  badge?: string;
}

defineProps<{
  visible: boolean;
  title: string;
  items: DetailItem[];
}>();
defineEmits<{ "update:visible": [value: boolean] }>();
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="title"
    :dismissable-mask="true"
    :pt="{ root: dialogRoot('max-w-xl') }"
    @update:visible="$emit('update:visible', $event)"
  >
    <dl
      class="grid max-h-[60vh] grid-cols-[minmax(7rem,auto)_1fr] gap-x-4 gap-y-2 overflow-auto p-1 text-sm"
    >
      <template v-for="item in items" :key="item.key">
        <dt class="text-surface-400">{{ item.label }}</dt>
        <dd
          class="min-w-0 wrap-break-word text-surface-700 dark:text-surface-200"
        >
          <span
            v-if="item.badge"
            class="inline-block rounded-full px-2 py-0.5 text-xs"
            :class="item.badge"
            >{{ item.text }}</span
          >
          <span v-else>{{ item.text }}</span>
        </dd>
      </template>
    </dl>
    <template #footer>
      <Button
        type="button"
        label="Close"
        severity="secondary"
        @click="$emit('update:visible', false)"
      />
    </template>
  </Dialog>
</template>
