<script setup lang="ts">
import { computed, ref, watch } from "vue";
import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import Slider from "primevue/slider";
import RadioButton from "primevue/radiobutton";
import Password from "primevue/password";
import Textarea from "primevue/textarea";
import Select from "primevue/select";
import MultiSelect from "primevue/multiselect";
import AutoComplete from "primevue/autocomplete";
import ToggleSwitch from "primevue/toggleswitch";
import Button from "primevue/button";
import FileUpload from "primevue/fileupload";
import type { FileUploadSelectEvent } from "primevue/fileupload";
import type {
  CredentialRefState,
  Field,
  Option,
  ResourceRef,
  Row,
  ValidatorType,
} from "../../types/projection";
import { fetchPage } from "../../api/dataSource";
import AppIcon from "../../components/AppIcon.vue";
import CredentialSelect from "./CredentialSelect.vue";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import ObjectField from "./ObjectField.vue";
import ArrayField from "./ArrayField.vue";
import MapField from "./MapField.vue";

const props = defineProps<{
  field: Field;
  modelValue: unknown;
  error?: string | null;
  secretSet?: boolean;
  credentialState?: CredentialRefState;
  protocol?: string;
  connectionId?: string;
  resource?: ResourceRef | null;
  hideLabel?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();

const fetchedOptions = ref<Option[] | null>(null);
const options = computed<Option[]>(
  () => fetchedOptions.value ?? props.field.options ?? [],
);

function rowOption(row: Row): Option {
  const r = row as Record<string, unknown>;
  const raw = r.value ?? r.name ?? r.column_name ?? r.column ?? r.key ?? "";
  const value =
    typeof raw === "number" || typeof raw === "boolean" ? raw : String(raw);
  return { value, label: String(r.label ?? r.name ?? value) };
}

watch(
  () => [props.field.optionsSource, props.connectionId, props.resource?.uid],
  async () => {
    const src = props.field.optionsSource;
    if (!src || !props.connectionId) return;
    try {
      const page = await fetchPage<Row>(
        props.connectionId,
        src,
        { resource: props.resource ?? null },
        { limit: 500 },
      );
      fetchedOptions.value = page.items.map(rowOption);
    } catch {
      fetchedOptions.value = [];
    }
  },
  { immediate: true },
);

const editingSecret = ref(false);
const showSecretValue = computed(
  () => !props.field.secret || !props.secretSet || editingSecret.value,
);

function bound(type: ValidatorType): number | undefined {
  const v = props.field.validators?.find((x) => x.type === type);
  const n = v === undefined ? NaN : Number(v.value);
  return Number.isFinite(n) ? n : undefined;
}
const min = computed(() => bound("min"));
const max = computed(() => bound("max"));
const step = computed(() => props.field.step ?? 1);
const sliderValue = computed(() =>
  typeof props.modelValue === "number" ? props.modelValue : (min.value ?? 0),
);

const TEXT_INPUT_TYPES: Record<string, string> = {
  email: "email",
  url: "url",
  tel: "tel",
};
const inputType = computed(() => TEXT_INPUT_TYPES[props.field.type] ?? "text");

function update(value: unknown): void {
  emit("update:modelValue", value);
}

const suggestions = ref<string[]>([]);
function onComplete(event: { query: string }): void {
  const all = options.value.map((o) => String(o.value));
  const q = event.query.trim().toLowerCase();
  suggestions.value = q ? all.filter((s) => s.toLowerCase().includes(q)) : all;
}

const jsonText = computed(() => {
  const v = props.modelValue;
  if (v === undefined || v === null) return "";
  if (typeof v === "string") return v;
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
});

function startReplace(): void {
  editingSecret.value = true;
  update("");
}

function updateFiles(event: FileUploadSelectEvent): void {
  const files = Array.isArray(event.files) ? event.files : [event.files];
  update(files.filter((file): file is File => file instanceof File));
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-1">
    <label
      v-if="!hideLabel"
      class="text-sm font-medium text-surface-700 dark:text-surface-200"
    >
      {{ field.label }}
      <span v-if="field.required" class="text-red-500">*</span>
    </label>

    <CredentialSelect
      v-if="field.type === 'credential_ref' && field.credential"
      :selector="field.credential"
      :protocol="protocol"
      :model-value="(modelValue as string) ?? ''"
      :state="credentialState"
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
      :options="options"
      option-label="label"
      option-value="value"
      :placeholder="field.placeholder ?? 'Select…'"
      @update:model-value="update"
    />

    <MultiSelect
      v-else-if="field.type === 'multiselect'"
      :model-value="(modelValue as unknown[]) ?? []"
      :options="options"
      option-label="label"
      option-value="value"
      display="chip"
      filter
      :max-selected-labels="3"
      :placeholder="field.placeholder ?? 'Select…'"
      @update:model-value="update"
    />

    <ToggleSwitch
      v-else-if="field.type === 'toggle'"
      :model-value="Boolean(modelValue)"
      @update:model-value="update"
    />

    <FileUpload
      v-else-if="field.type === 'file'"
      mode="basic"
      custom-upload
      :multiple="true"
      choose-label="Choose file"
      @select="updateFiles"
    />

    <div
      v-else-if="field.type === 'json'"
      class="h-56 overflow-hidden rounded-md border border-surface-300 dark:border-surface-700"
    >
      <CodeTextEditor
        :value="jsonText"
        language="json"
        :aria-label="field.label"
        @update:value="update"
      />
    </div>

    <Textarea
      v-else-if="field.type === 'textarea'"
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
      :use-grouping="false"
      :min="min"
      :max="max"
      :step="step"
      @update:model-value="update"
    />

    <InputNumber
      v-else-if="field.type === 'stepper'"
      :model-value="(modelValue as number) ?? null"
      :use-grouping="false"
      show-buttons
      button-layout="horizontal"
      :min="min"
      :max="max"
      :step="step"
      @update:model-value="update"
    >
      <template #incrementicon>
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
      </template>
      <template #decrementicon>
        <AppIcon :icon="{ type: 'lucide', value: 'minus' }" :size="14" />
      </template>
    </InputNumber>

    <div v-else-if="field.type === 'slider'" class="flex items-center gap-3">
      <Slider
        :model-value="sliderValue"
        :min="min ?? 0"
        :max="max ?? 100"
        :step="step"
        class="min-w-0 flex-1"
        @update:model-value="update"
      />
      <span
        class="w-12 shrink-0 text-right text-sm text-surface-700 tabular-nums dark:text-surface-200"
        >{{ sliderValue }}</span
      >
    </div>

    <div v-else-if="field.type === 'radio'" class="flex flex-col gap-2 pt-1">
      <label
        v-for="opt in options"
        :key="String(opt.value)"
        class="flex items-center gap-2 text-sm text-surface-700 dark:text-surface-200"
      >
        <RadioButton
          :model-value="modelValue"
          :value="opt.value"
          :input-id="`${field.key}-${opt.value}`"
          @update:model-value="update"
        />
        <span>{{ opt.label }}</span>
      </label>
    </div>

    <AutoComplete
      v-else-if="field.type === 'autocomplete'"
      :model-value="(modelValue as string) ?? ''"
      :suggestions="suggestions"
      dropdown
      :placeholder="field.placeholder"
      @complete="onComplete"
      @update:model-value="update"
    />

    <ObjectField
      v-else-if="field.type === 'object'"
      :field="field"
      :model-value="modelValue"
      :connection-id="connectionId"
      :resource="resource"
      @update:model-value="update"
    />

    <ArrayField
      v-else-if="field.type === 'array'"
      :field="field"
      :model-value="modelValue"
      :connection-id="connectionId"
      :resource="resource"
      @update:model-value="update"
    />

    <MapField
      v-else-if="field.type === 'map'"
      :field="field"
      :model-value="modelValue"
      :connection-id="connectionId"
      :resource="resource"
      @update:model-value="update"
    />

    <InputText
      v-else-if="field.type === 'duration'"
      :model-value="(modelValue as string) ?? ''"
      :placeholder="field.placeholder ?? '30s, 5m, 1h'"
      @update:model-value="update"
    />

    <InputText
      v-else
      :type="inputType"
      :model-value="(modelValue as string) ?? ''"
      :placeholder="field.placeholder"
      @update:model-value="update"
    />

    <p v-if="field.help" class="text-xs text-surface-400">{{ field.help }}</p>
    <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
  </div>
</template>
