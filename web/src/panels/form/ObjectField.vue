<script setup lang="ts">
import { computed } from "vue";
import type { Field, ResourceRef } from "@/types/projection";
import FieldGroup from "./FieldGroup.vue";

const props = defineProps<{
  field: Field;
  modelValue: unknown;
  connectionId?: string;
  resource?: ResourceRef | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
}>();

const record = computed(
  () => (props.modelValue ?? {}) as Record<string, unknown>,
);

function set(key: string, value: unknown): void {
  emit("update:modelValue", { ...record.value, [key]: value });
}
</script>

<template>
  <div
    class="flex min-w-0 flex-col gap-4 rounded-md border border-surface-200 p-3 dark:border-surface-800"
  >
    <FieldGroup
      :fields="field.fields ?? []"
      :values="record"
      :connection-id="connectionId"
      :resource="resource"
      @update="set"
    />
  </div>
</template>
