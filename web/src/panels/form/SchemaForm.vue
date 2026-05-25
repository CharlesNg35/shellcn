<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import type { Field, Schema } from "../../types/projection";
import FormField from "./FormField.vue";
import { isVisible, validateField } from "./condition";

const props = defineProps<{
  schema: Schema;
  modelValue?: Record<string, unknown>;
  secretsSet?: Record<string, boolean>;
  protocol?: string;
  submitLabel?: string;
  busy?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
  submit: [value: Record<string, unknown>];
}>();

const values = reactive<Record<string, unknown>>({});
const errors = ref<Record<string, string>>({});

function seed(): void {
  for (const group of props.schema.groups) {
    for (const field of group.fields) {
      const incoming = props.modelValue?.[field.key];
      values[field.key] = incoming !== undefined ? incoming : field.default;
    }
  }
}
watch(() => props.schema, seed, { immediate: true });

function set(field: Field, value: unknown): void {
  values[field.key] = value;
  delete errors.value[field.key];
  emit("update:modelValue", { ...values });
}

function visibleFields(fields: Field[]): Field[] {
  return fields.filter((f) => isVisible(f.visibleWhen, values));
}

function onSubmit(): void {
  const next: Record<string, string> = {};
  const payload: Record<string, unknown> = {};
  for (const group of props.schema.groups) {
    for (const field of visibleFields(group.fields)) {
      const value = values[field.key];
      const msg = validateField(field, value);
      if (msg) next[field.key] = msg;
      else if (value !== undefined) payload[field.key] = value;
    }
  }
  errors.value = next;
  if (Object.keys(next).length === 0) emit("submit", payload);
}
</script>

<template>
  <form class="flex flex-col gap-6" @submit.prevent="onSubmit">
    <fieldset
      v-for="group in schema.groups"
      :key="group.name"
      class="flex flex-col gap-4"
    >
      <legend
        class="text-xs font-semibold uppercase tracking-wide text-surface-400"
      >
        {{ group.name }}
      </legend>
      <FormField
        v-for="field in visibleFields(group.fields)"
        :key="field.key"
        :field="field"
        :model-value="values[field.key]"
        :error="errors[field.key]"
        :secret-set="secretsSet?.[field.key]"
        :protocol="protocol"
        @update:model-value="set(field, $event)"
      />
    </fieldset>

    <div v-if="submitLabel" class="flex justify-end">
      <button
        type="submit"
        :disabled="busy"
        class="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50"
      >
        {{ busy ? "Working…" : submitLabel }}
      </button>
    </div>
  </form>
</template>
