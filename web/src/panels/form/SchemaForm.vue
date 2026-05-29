<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import Button from "primevue/button";
import type {
  CredentialRefState,
  Field,
  ResourceRef,
  Schema,
} from "../../types/projection";
import FormField from "./FormField.vue";
import { isVisible, validateField } from "./condition";

const props = defineProps<{
  schema: Schema;
  modelValue?: Record<string, unknown>;
  secretsSet?: Record<string, boolean>;
  credentialStates?: Record<string, CredentialRefState>;
  context?: Record<string, unknown>;
  protocol?: string;
  submitLabel?: string;
  busy?: boolean;
  // Forwarded to fields with a route-sourced options list (optionsSource).
  connectionId?: string;
  resource?: ResourceRef | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
  submit: [
    value: Record<string, unknown>,
    meta: { preserveCredentials: string[] },
  ];
}>();

const values = reactive<Record<string, unknown>>({});
const errors = ref<Record<string, string>>({});
const touched = ref<Record<string, boolean>>({});

// A plugin may declare a config schema with no groups. Normalize so every
// consumer iterates safely.
const groups = computed(() => props.schema?.groups ?? []);
const conditionValues = computed(() => ({
  ...values,
  ...(props.context ?? {}),
}));
const visibleGroups = computed(() =>
  groups.value
    .map((group) => ({ ...group, fields: visibleFields(group.fields ?? []) }))
    .filter((group) => group.fields.length > 0),
);

function seed(resetTouched = true): void {
  if (resetTouched) touched.value = {};
  const next: Record<string, unknown> = {};
  for (const group of groups.value) {
    for (const field of group.fields ?? []) {
      const incoming = props.modelValue?.[field.key];
      next[field.key] = incoming !== undefined ? incoming : field.default;
    }
  }
  for (const key of Object.keys(values)) {
    if (!(key in next)) delete values[key];
  }
  Object.assign(values, next);
  emit("update:modelValue", { ...values });
}

function modelMatchesValues(model?: Record<string, unknown>): boolean {
  for (const group of groups.value) {
    for (const field of group.fields ?? []) {
      const incoming = model?.[field.key];
      const expected = incoming !== undefined ? incoming : field.default;
      if (values[field.key] !== expected) return false;
    }
  }
  return true;
}

watch(
  () => props.schema,
  () => seed(true),
  { immediate: true },
);
watch(
  () => props.modelValue,
  (model) => {
    if (!modelMatchesValues(model)) seed(true);
  },
  { deep: true },
);

function set(field: Field, value: unknown): void {
  touched.value = { ...touched.value, [field.key]: true };
  values[field.key] = value;
  delete errors.value[field.key];
  emit("update:modelValue", { ...values });
}

function visibleFields(fields: Field[]): Field[] {
  return fields.filter((f) => isVisible(f.visibleWhen, conditionValues.value));
}

function isBlank(value: unknown): boolean {
  return (
    value === undefined ||
    value === null ||
    value === "" ||
    (Array.isArray(value) && value.length === 0)
  );
}

function onSubmit(): void {
  const next: Record<string, string> = {};
  const payload: Record<string, unknown> = {};
  const preserveCredentials: string[] = [];
  for (const group of groups.value) {
    for (const field of visibleFields(group.fields ?? [])) {
      let value = values[field.key];
      // A write-only secret that is already set and left untouched is kept by
      // the backend — never require or resubmit it.
      if (field.secret && props.secretsSet?.[field.key] && isBlank(value)) {
        continue;
      }
      // JSON fields edit as text; parse to an object so it binds server-side.
      if (field.type === "json" && typeof value === "string") {
        const trimmed = value.trim();
        if (trimmed === "") {
          value = undefined;
        } else {
          try {
            value = JSON.parse(trimmed);
          } catch {
            next[field.key] = "Enter valid JSON.";
            continue;
          }
        }
      }
      const credentialState = props.credentialStates?.[field.key];
      if (
        field.type === "credential_ref" &&
        credentialState?.state === "set" &&
        !credentialState.readable &&
        !touched.value[field.key] &&
        isBlank(value)
      ) {
        preserveCredentials.push(field.key);
        continue;
      }
      const msg = validateField(field, value);
      if (msg) next[field.key] = msg;
      else if (value !== undefined) payload[field.key] = value;
    }
  }
  errors.value = next;
  if (Object.keys(next).length === 0) {
    emit("submit", payload, { preserveCredentials });
  }
}

defineExpose({ submit: onSubmit });
</script>

<template>
  <form class="flex min-w-0 flex-col gap-6" @submit.prevent="onSubmit">
    <fieldset
      v-for="group in visibleGroups"
      :key="group.name"
      class="flex min-w-0 flex-col gap-4"
    >
      <legend
        class="text-xs font-semibold tracking-wide text-surface-400 uppercase"
      >
        {{ group.name }}
      </legend>
      <FormField
        v-for="field in group.fields"
        :key="field.key"
        :field="field"
        :model-value="values[field.key]"
        :error="errors[field.key]"
        :secret-set="secretsSet?.[field.key]"
        :credential-state="credentialStates?.[field.key]"
        :protocol="protocol"
        :connection-id="connectionId"
        :resource="resource"
        @update:model-value="set(field, $event)"
      />
    </fieldset>

    <div v-if="submitLabel" class="flex justify-end">
      <Button
        type="submit"
        :label="submitLabel"
        :loading="busy"
        :disabled="busy"
      />
    </div>
  </form>
</template>
