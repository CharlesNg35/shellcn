<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import type { Field, ResourceRef } from "@/types/projection";
import FieldGroup from "./FieldGroup.vue";
import FormField from "./FormField.vue";
import AppIcon from "@/components/AppIcon.vue";
import { defaultForField } from "./defaults";

const props = defineProps<{
  field: Field;
  modelValue: unknown;
  connectionId?: string;
  resource?: ResourceRef | null;
}>();
const emit = defineEmits<{ "update:modelValue": [value: unknown[]] }>();

const items = computed(() =>
  Array.isArray(props.modelValue) ? props.modelValue : [],
);
const item = computed(() => props.field.item);
const isObjectItem = computed(() => item.value?.type === "object");
const rowLabel = computed(() => props.field.itemLabel ?? "Item");
const atMax = computed(
  () =>
    Boolean(props.field.maxItems) &&
    items.value.length >= (props.field.maxItems ?? 0),
);
const atMin = computed(() => items.value.length <= (props.field.minItems ?? 0));

function add(): void {
  if (!item.value || atMax.value) return;
  emit("update:modelValue", [...items.value, defaultForField(item.value)]);
}
function removeAt(index: number): void {
  emit(
    "update:modelValue",
    items.value.filter((_, i) => i !== index),
  );
}
function setObject(index: number, key: string, value: unknown): void {
  const next = items.value.slice();
  next[index] = {
    ...((next[index] ?? {}) as Record<string, unknown>),
    [key]: value,
  };
  emit("update:modelValue", next);
}
function setScalar(index: number, value: unknown): void {
  const next = items.value.slice();
  next[index] = value;
  emit("update:modelValue", next);
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-2">
    <p v-if="!items.length" class="text-xs text-surface-400">
      No {{ rowLabel.toLowerCase() }}s yet.
    </p>

    <div
      v-for="(row, index) in items"
      :key="index"
      class="flex min-w-0 items-start gap-2 rounded-md border border-surface-200 p-3 motion-safe:transition-colors dark:border-surface-800"
    >
      <div class="min-w-0 flex-1">
        <FieldGroup
          v-if="isObjectItem && item"
          :fields="item.fields ?? []"
          :values="(row ?? {}) as Record<string, unknown>"
          :connection-id="connectionId"
          :resource="resource"
          @update="(key, value) => setObject(index, key, value)"
        />
        <FormField
          v-else-if="item"
          :field="{ ...item, label: `${rowLabel} ${index + 1}` }"
          :model-value="row"
          :connection-id="connectionId"
          :resource="resource"
          @update:model-value="(value) => setScalar(index, value)"
        />
      </div>
      <Button
        type="button"
        severity="danger"
        text
        :disabled="atMin"
        :aria-label="`Remove ${rowLabel} ${index + 1}`"
        @click="removeAt(index)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'trash-2' }" :size="15" />
      </Button>
    </div>

    <div>
      <Button
        type="button"
        severity="secondary"
        size="small"
        :disabled="atMax"
        @click="add"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
        {{ field.addLabel ?? `Add ${rowLabel}` }}
      </Button>
    </div>
  </div>
</template>
