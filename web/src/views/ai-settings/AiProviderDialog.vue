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
import {
  defaultProviderName,
  providerKindOptions,
  requiresBaseUrl,
} from "./providerKinds";

const props = defineProps<{
  visible: boolean;
  providers: AiProviderSummary[];
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
  model: string;
}>({
  kind: "openrouter",
  name: "",
  baseUrl: "",
  apiKey: "",
  model: "",
});

const busy = ref(false);
const fetchingModels = ref(false);
const testingProvider = ref(false);
const formError = ref("");
const testStatus = ref<{ ok: boolean; message: string } | null>(null);
const modelOptions = ref<string[]>([]);
const filteredModels = ref<string[]>([]);

const isEdit = computed(() => Boolean(props.provider));
const needsBaseUrl = computed(() => requiresBaseUrl(form.kind));
const keyPlaceholder = computed(() =>
  isEdit.value ? "Leave blank to keep saved key" : "Provider API key",
);
const canUseSavedProvider = computed(() => {
  const p = props.provider;
  return Boolean(
    p &&
    !form.apiKey.trim() &&
    form.kind === p.kind &&
    form.baseUrl.trim() === (p.baseUrl ?? "").trim(),
  );
});

function reset(): void {
  const p = props.provider;
  if (p) {
    Object.assign(form, {
      kind: p.kind,
      name: p.name,
      baseUrl: p.baseUrl ?? "",
      apiKey: "",
      model: p.model,
    });
    modelOptions.value = [...p.models];
  } else {
    Object.assign(form, {
      kind: "openrouter",
      name: defaultProviderName("openrouter"),
      baseUrl: "",
      apiKey: "",
      model: "",
    });
    modelOptions.value = [];
  }
  filteredModels.value = [...modelOptions.value];
  formError.value = "";
  testStatus.value = null;
}

watch(
  () => props.visible,
  (open) => {
    if (open) reset();
  },
  { immediate: true },
);

function applyKind(kind: AiProviderKind): void {
  form.kind = kind;
  if (!isEdit.value || !form.name.trim()) {
    form.name = defaultProviderName(kind);
  }
  if (!requiresBaseUrl(kind)) {
    form.baseUrl = "";
  }
  form.model = "";
  modelOptions.value = [];
  filteredModels.value = [...modelOptions.value];
  testStatus.value = null;
}

function buildInput(): AiProviderInput {
  const models = [...modelOptions.value];
  if (form.model && !models.includes(form.model)) {
    models.unshift(form.model);
  }
  return {
    kind: form.kind,
    name: form.name.trim(),
    baseUrl: form.baseUrl.trim() || undefined,
    apiKey: form.apiKey.trim() || undefined,
    models,
    model: form.model.trim(),
  };
}

async function fetchModels(): Promise<void> {
  formError.value = "";
  testStatus.value = null;
  if (needsBaseUrl.value && !form.baseUrl.trim()) {
    formError.value = "Base URL is required";
    return;
  }
  fetchingModels.value = true;
  try {
    const source = canUseSavedProvider.value
      ? await aiApi.models(props.provider!.id)
      : await aiApi.previewModels(buildInput());
    modelOptions.value = source.models;
    filteredModels.value = source.models;
    if (!form.model && source.models[0]) {
      form.model = source.models[0];
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

function providerNameExists(name: string): boolean {
  const normalized = name.trim().toLowerCase();
  return props.providers.some(
    (provider) =>
      provider.id !== props.provider?.id &&
      provider.name.trim().toLowerCase() === normalized,
  );
}

async function testProvider(): Promise<void> {
  formError.value = "";
  testStatus.value = null;
  if (needsBaseUrl.value && !form.baseUrl.trim()) {
    formError.value = "Base URL is required";
    return;
  }
  if (!form.model.trim()) {
    formError.value = "Choose or type a model";
    return;
  }
  testingProvider.value = true;
  try {
    const result = canUseSavedProvider.value
      ? await aiApi.testProvider(props.provider!.id)
      : await aiApi.testProviderDraft(buildInput());
    testStatus.value = {
      ok: result.ok,
      message: result.ok
        ? "Connection OK"
        : (result.error ?? "Provider test failed"),
    };
  } catch (err) {
    formError.value =
      err instanceof ApiError ? err.message : "Failed to test provider";
  } finally {
    testingProvider.value = false;
  }
}

async function save(): Promise<void> {
  formError.value = "";
  testStatus.value = null;
  if (!form.name.trim()) {
    formError.value = "Provider name is required";
    return;
  }
  if (providerNameExists(form.name)) {
    formError.value = "Provider name already exists";
    return;
  }
  if (needsBaseUrl.value && !form.baseUrl.trim()) {
    formError.value = "Base URL is required";
    return;
  }
  if (!form.model.trim()) {
    formError.value = "Choose or type a model";
    return;
  }
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
          :options="providerKindOptions"
          option-label="label"
          option-value="value"
          @update:model-value="applyKind($event)"
        />
      </div>

      <div class="grid min-w-0 gap-1.5">
        <label
          for="ai-provider-name"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Name
        </label>
        <InputText
          id="ai-provider-name"
          :model-value="form.name"
          placeholder="Provider display name"
          @update:model-value="form.name = $event ?? ''"
        />
      </div>

      <div v-if="needsBaseUrl" class="grid min-w-0 gap-1.5">
        <label
          for="ai-provider-base-url"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Base URL <span class="text-red-500">*</span>
        </label>
        <InputText
          id="ai-provider-base-url"
          :model-value="form.baseUrl"
          placeholder="http://127.0.0.1:11434/v1"
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
            Model <span class="text-red-500">*</span>
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
          :model-value="form.model"
          :suggestions="filteredModels"
          dropdown
          placeholder="Select or type a model"
          @complete="searchModels"
          @update:model-value="form.model = String($event ?? '')"
          @dropdown-click="fetchModels"
        />
        <p class="text-xs text-surface-400">
          This model is required and will be used by conversations that select
          this provider.
        </p>
      </div>

      <p v-if="formError" class="text-sm text-red-500">{{ formError }}</p>
    </div>

    <template #footer>
      <div class="flex w-full min-w-0 items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <Button
            severity="secondary"
            outlined
            size="small"
            :loading="testingProvider"
            :disabled="busy || fetchingModels"
            data-testid="test-ai-provider"
            @click="testProvider"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'plug-zap' }" :size="14" />
            Test provider
          </Button>
          <p
            v-if="testStatus"
            class="truncate text-xs"
            :class="
              testStatus.ok
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-red-500'
            "
            role="status"
          >
            {{ testStatus.message }}
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <Button
            :pt="{ root: btnGhost }"
            :disabled="busy"
            @click="visible = false"
          >
            Cancel
          </Button>
          <Button
            :pt="{ root: btnPrimary }"
            :loading="busy"
            data-testid="save-ai-provider"
            @click="save"
          >
            {{ isEdit ? "Save" : "Add provider" }}
          </Button>
        </div>
      </div>
    </template>
  </Dialog>
</template>
