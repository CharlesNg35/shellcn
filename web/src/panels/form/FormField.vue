<script setup lang="ts">
import { computed, ref } from "vue";
import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import Password from "primevue/password";
import Textarea from "primevue/textarea";
import Select from "primevue/select";
import ToggleSwitch from "primevue/toggleswitch";
import Button from "primevue/button";
import type { Field } from "../../types/projection";
import CredentialSelect from "./CredentialSelect.vue";

const props = defineProps<{
  field: Field;
  modelValue: unknown;
  error?: string | null;
  secretSet?: boolean;
  protocol?: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();

const editingSecret = ref(false);
const showSecretValue = computed(
  () => !props.field.secret || !props.secretSet || editingSecret.value,
);

function update(value: unknown): void {
  emit("update:modelValue", value);
}

function startReplace(): void {
  editingSecret.value = true;
  update("");
}
</script>

<template>
  <div class="flex flex-col gap-1">
    <label class="text-sm font-medium text-surface-700 dark:text-surface-200">
      {{ field.label }}
      <span v-if="field.required" class="text-red-500">*</span>
    </label>

    <CredentialSelect
      v-if="field.type === 'credential_ref' && field.credential"
      :selector="field.credential"
      :protocol="protocol"
      :model-value="(modelValue as string) ?? ''"
      @update:model-value="update"
    />

    <Button
      v-else-if="field.secret && !showSecretValue"
      type="button"
      :pt="{
        root: 'flex w-full items-center justify-between rounded-md border border-surface-300 px-2.5 py-1.5 text-sm text-surface-500 dark:border-surface-700',
      }"
      @click="startReplace"
    >
      <span>•••••••• Set</span>
      <span class="text-xs text-primary-500">Replace</span>
    </Button>

    <Select
      v-else-if="field.type === 'select'"
      :model-value="modelValue"
      :options="field.options"
      option-label="label"
      option-value="value"
      :placeholder="field.placeholder ?? 'Select…'"
      @update:model-value="update"
    />

    <ToggleSwitch
      v-else-if="field.type === 'toggle'"
      :model-value="Boolean(modelValue)"
      @update:model-value="update"
    />

    <Textarea
      v-else-if="field.type === 'textarea' || field.type === 'json'"
      :model-value="(modelValue as string) ?? ''"
      rows="4"
      :placeholder="field.placeholder"
      @update:model-value="update"
    />

    <Password
      v-else-if="field.type === 'password'"
      :model-value="(modelValue as string) ?? ''"
      :feedback="false"
      toggle-mask
      :input-props="{ autocomplete: 'new-password' }"
      @update:model-value="update"
    />

    <InputNumber
      v-else-if="field.type === 'number'"
      :model-value="(modelValue as number) ?? null"
      @update:model-value="update"
    />

    <InputText
      v-else
      :model-value="(modelValue as string) ?? ''"
      :placeholder="field.placeholder"
      @update:model-value="update"
    />

    <p v-if="field.help" class="text-xs text-surface-400">{{ field.help }}</p>
    <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
  </div>
</template>
