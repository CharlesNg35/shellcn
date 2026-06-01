<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import AutoComplete from "primevue/autocomplete";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Select from "primevue/select";
import { ApiError } from "../../api/client";
import {
  aiApi,
  type AiProviderInput,
  type AiProviderKind,
  type AiProviderSummary,
} from "../../api/ai";
import AppIcon from "../../components/AppIcon.vue";
import { btnGhost, btnPrimary, dialogRoot } from "../../primevue/preset";
import { providerPreset, providerPresets } from "./providerCatalog";

const props = defineProps<{
  visible: boolean;
  provider?: AiProviderSummary | null;
}>();

const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [];
}>();

const visible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit("update:visible", value),
});

const form = reactive<{
  kind: AiProviderKind;
  name: string;
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
}>({
  kind: "openrouter",
  name: "",
  baseUrl: "",
  apiKey: "",
  defaultModel: "",
});

const busy = ref(false);
const fetchingModels = ref(false);
const formError = ref("");
const modelOptions = ref<string[]>([]);
const filteredModels = ref<string[]>([]);

const kindOptions = providerPresets.map((p) => ({
  label: p.label,
  value: p.kind,
}));

const isEdit = computed(() => Boolean(props.provider));
const preset = computed(() => providerPreset(form.kind));
const isCustomProvider = computed(() => Boolean(preset.value.custom));
const needsBaseUrl = computed(() => Boolean(preset.value.requiresBaseUrl));
const keyPlaceholder = computed(() =>
  isEdit.value ? "Leave blank to keep saved key" : "Provider API key",
);

function reset(): void {
  const p = props.provider;
  if (p) {
    Object.assign(form, {
      kind: p.kind,
      name: p.name,
      baseUrl: p.baseUrl ?? "",
      apiKey: "",
      defaultModel: p.defaultModel,
    });
    modelOptions.value = [...p.models];
  } else {
    const next = providerPreset("openrouter");
    Object.assign(form, {
      kind: next.kind,
      name: next.defaultName,
      baseUrl: next.baseUrl ?? "",
      apiKey: "",
      defaultModel: next.defaultModel,
    });
    modelOptions.value = [...next.models];
  }
  filteredModels.value = [...modelOptions.value];
  formError.value = "";
}

watch(
  () => props.visible,
  (open) => {
    if (open) reset();
  },
);

function applyKind(kind: AiProviderKind): void {
  const next = providerPreset(kind);
  form.kind = kind;
  if (!isEdit.value || !form.name.trim() || !next.custom) {
    form.name = next.defaultName;
  }
  form.baseUrl = next.baseUrl ?? "";
  form.defaultModel = next.defaultModel;
  modelOptions.value = [...next.models];
  filteredModels.value = [...modelOptions.value];
}

function buildInput(): AiProviderInput {
  const models = [...modelOptions.value];
  if (form.defaultModel && !models.includes(form.defaultModel)) {
    models.unshift(form.defaultModel);
  }
  return {
    kind: form.kind,
    name: isCustomProvider.value ? form.name.trim() : preset.value.defaultName,
    baseUrl: form.baseUrl.trim() || undefined,
    apiKey: form.apiKey.trim() || undefined,
    models,
    defaultModel: form.defaultModel.trim(),
  };
}

async function fetchModels(): Promise<void> {
  formError.value = "";
  fetchingModels.value = true;
  try {
    const source =
      props.provider && !form.apiKey.trim()
        ? await aiApi.models(props.provider.id)
        : await aiApi.previewModels(buildInput());
    modelOptions.value = source.models;
    filteredModels.value = source.models;
    if (!form.defaultModel && source.models[0]) {
      form.defaultModel = source.models[0];
    }
  } catch (err) {
    formError.value =
      err instanceof ApiError ? err.message : "Failed to fetch models";
  } finally {
    fetchingModels.value = false;
  }
}

function searchModels(event: { query: string }): void {
  const query = event.query.trim().toLowerCase();
  if (!query) {
    filteredModels.value = [...modelOptions.value];
    return;
  }
  filteredModels.value = modelOptions.value.filter((model) =>
    model.toLowerCase().includes(query),
  );
}

async function save(): Promise<void> {
  formError.value = "";
  busy.value = true;
  try {
    const input = buildInput();
    if (props.provider) {
      await aiApi.update(props.provider.id, input);
    } else {
      await aiApi.create(input);
    }
    emit("saved");
    visible.value = false;
  } catch (err) {
    formError.value =
      err instanceof ApiError ? err.message : "Failed to save provider";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    :header="isEdit ? 'Edit provider' : 'Add provider'"
    :closable="!busy"
    :pt="{
      root: dialogRoot('max-w-xl'),
      content: 'min-h-0 max-h-[72vh] overflow-auto p-5',
    }"
  >
    <div class="grid min-w-0 gap-4">
      <div class="grid min-w-0 gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Provider
        </label>
        <Select
          :model-value="form.kind"
          :options="kindOptions"
          option-label="label"
          option-value="value"
          @update:model-value="applyKind($event)"
        />
      </div>

      <div v-if="isCustomProvider" class="grid min-w-0 gap-1.5">
        <label
          for="ai-provider-name"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Name
        </label>
        <InputText
          id="ai-provider-name"
          :model-value="form.name"
          @update:model-value="form.name = $event ?? ''"
        />
      </div>

      <div v-if="needsBaseUrl" class="grid min-w-0 gap-1.5">
        <label
          for="ai-provider-base-url"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Base URL
        </label>
        <InputText
          id="ai-provider-base-url"
          :model-value="form.baseUrl"
          placeholder="https://host/v1"
          @update:model-value="form.baseUrl = $event ?? ''"
        />
      </div>

      <div class="grid min-w-0 gap-1.5">
        <label
          for="ai-provider-key"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          API key
        </label>
        <Password
          input-id="ai-provider-key"
          :model-value="form.apiKey"
          :feedback="false"
          toggle-mask
          :placeholder="keyPlaceholder"
          fluid
          @update:model-value="form.apiKey = $event ?? ''"
        />
      </div>

      <div class="grid min-w-0 gap-1.5">
        <div class="flex items-center justify-between gap-3">
          <label
            for="ai-provider-model"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Model
          </label>
          <Button
            size="small"
            text
            severity="secondary"
            :loading="fetchingModels"
            :disabled="fetchingModels"
            @click="fetchModels"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'refresh-cw' }"
              :size="14"
            />
            Fetch models
          </Button>
        </div>
        <AutoComplete
          input-id="ai-provider-model"
          :model-value="form.defaultModel"
          :suggestions="filteredModels"
          dropdown
          placeholder="Select or type a model"
          @complete="searchModels"
          @update:model-value="form.defaultModel = String($event ?? '')"
          @dropdown-click="fetchModels"
        />
        <p class="text-xs text-surface-400">
          Choose from the provider list, or type a model name directly.
        </p>
      </div>

      <p v-if="formError" class="text-sm text-red-500">{{ formError }}</p>
    </div>

    <template #footer>
      <Button
        :pt="{ root: btnGhost }"
        :disabled="busy"
        @click="visible = false"
      >
        Cancel
      </Button>
      <Button :pt="{ root: btnPrimary }" :loading="busy" @click="save">
        {{ isEdit ? "Save" : "Add provider" }}
      </Button>
    </template>
  </Dialog>
</template>
