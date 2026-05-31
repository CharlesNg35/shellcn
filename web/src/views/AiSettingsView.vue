<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Tag from "primevue/tag";
import { ApiError } from "../api/client";
import {
  aiApi,
  type AiGlobalStatus,
  type AiProviderInput,
  type AiProviderKind,
  type AiProviderSummary,
} from "../api/ai";
import { useNotify } from "../composables/useNotify";
import { useConfirmAction } from "../composables/useConfirmAction";
import AppIcon from "../components/AppIcon.vue";
import { dialogRoot, btnPrimary, btnGhost } from "../primevue/preset";

const notify = useNotify();
const { confirmDanger } = useConfirmAction();

const loading = ref(true);
const providers = ref<AiProviderSummary[]>([]);
const global = ref<AiGlobalStatus | null>(null);

const kindOptions: { label: string; value: AiProviderKind }[] = [
  { label: "OpenAI", value: "openai" },
  { label: "Anthropic", value: "anthropic" },
  { label: "Google", value: "google" },
  { label: "OpenAI-compatible (custom)", value: "openai_compatible" },
];

const dialogOpen = ref(false);
const editingId = ref<string | null>(null);
const busy = ref(false);
const formError = ref("");
const form = reactive<{
  kind: AiProviderKind;
  name: string;
  baseUrl: string;
  apiKey: string;
  models: string;
  defaultModel: string;
}>({
  kind: "openai",
  name: "",
  baseUrl: "",
  apiKey: "",
  models: "",
  defaultModel: "",
});

const isEdit = computed(() => editingId.value !== null);
const needsBaseUrl = computed(() => form.kind === "openai_compatible");
const keyPlaceholder = computed(() =>
  isEdit.value ? "•••••••• (leave blank to keep)" : "Provider API key",
);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const [g, list] = await Promise.all([aiApi.global(), aiApi.list()]);
    global.value = g;
    providers.value = list;
  } catch (err) {
    notify.error(
      "Failed to load AI settings",
      err instanceof ApiError ? err.message : undefined,
    );
  } finally {
    loading.value = false;
  }
}

function openCreate(): void {
  editingId.value = null;
  formError.value = "";
  Object.assign(form, {
    kind: "openai" as AiProviderKind,
    name: "",
    baseUrl: "",
    apiKey: "",
    models: "",
    defaultModel: "",
  });
  dialogOpen.value = true;
}

function openEdit(p: AiProviderSummary): void {
  editingId.value = p.id;
  formError.value = "";
  Object.assign(form, {
    kind: p.kind,
    name: p.name,
    baseUrl: p.baseUrl ?? "",
    apiKey: "",
    models: p.models.join(", "),
    defaultModel: p.defaultModel,
  });
  dialogOpen.value = true;
}

function buildInput(): AiProviderInput {
  const models = form.models
    .split(",")
    .map((m) => m.trim())
    .filter(Boolean);
  return {
    kind: form.kind,
    name: form.name.trim(),
    baseUrl: form.baseUrl.trim() || undefined,
    apiKey: form.apiKey.trim() || undefined,
    models,
    defaultModel: form.defaultModel.trim(),
  };
}

async function save(): Promise<void> {
  formError.value = "";
  busy.value = true;
  try {
    const input = buildInput();
    if (editingId.value) {
      await aiApi.update(editingId.value, input);
      notify.success("Provider updated");
    } else {
      await aiApi.create(input);
      notify.success("Provider added");
    }
    dialogOpen.value = false;
    await load();
  } catch (err) {
    formError.value =
      err instanceof ApiError ? err.message : "Failed to save provider";
  } finally {
    busy.value = false;
  }
}

function remove(p: AiProviderSummary): void {
  confirmDanger({
    header: "Delete provider",
    message: `Delete "${p.name}"? Conversations using it will fall back to another configured provider.`,
    accept: async () => {
      try {
        await aiApi.remove(p.id);
        notify.success("Provider deleted");
        await load();
      } catch (err) {
        notify.error(
          "Failed to delete",
          err instanceof ApiError ? err.message : undefined,
        );
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="mx-auto flex max-w-2xl flex-col gap-4 p-8">
    <div class="flex items-center gap-2">
      <RouterLink
        :to="{ name: 'settings' }"
        class="rounded-md p-1 text-surface-400 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800"
        aria-label="Back to settings"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'arrow-left' }" :size="18" />
      </RouterLink>
      <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
        AI providers
      </h1>
    </div>

    <p class="text-sm text-surface-500 dark:text-surface-400">
      Configure your own AI providers — built-in vendors or any
      OpenAI-compatible endpoint. Keys are encrypted and never shown again.
    </p>

    <!-- Read-only shared-AI indicator (env/config, not editable here). -->
    <div
      v-if="global?.configured"
      class="flex items-center gap-3 rounded-lg border border-surface-200 px-4 py-3 dark:border-surface-800"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'bot' }"
        :size="18"
        class="text-surface-400"
      />
      <div class="flex min-w-0 flex-1 flex-col">
        <p class="font-medium text-surface-800 dark:text-surface-100">
          Shared AI
        </p>
        <p class="truncate text-xs text-surface-500 dark:text-surface-400">
          {{ global.provider }} · {{ global.model }} (managed by your operator)
        </p>
      </div>
      <Tag value="Shared" severity="secondary" />
    </div>

    <!-- Loading skeleton. -->
    <div v-if="loading" class="flex flex-col gap-3" aria-busy="true">
      <div
        v-for="i in 2"
        :key="i"
        class="h-16 animate-pulse rounded-lg border border-surface-200 bg-surface-100/60 dark:border-surface-800 dark:bg-surface-800/40"
      />
    </div>

    <template v-else>
      <!-- Empty state. -->
      <div
        v-if="providers.length === 0"
        class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-surface-300 px-4 py-10 text-center dark:border-surface-700"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'sparkles' }"
          :size="28"
          class="text-surface-300"
        />
        <p class="text-sm text-surface-500 dark:text-surface-400">
          No AI providers yet. Add one to use the assistant with your own key.
        </p>
        <Button :pt="{ root: btnPrimary }" @click="openCreate">
          Add provider
        </Button>
      </div>

      <!-- Provider list. -->
      <ul v-else class="flex flex-col gap-3">
        <li
          v-for="p in providers"
          :key="p.id"
          class="flex items-center gap-3 rounded-lg border border-surface-200 px-4 py-3 dark:border-surface-800"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'sparkles' }"
            :size="18"
            class="text-surface-400"
          />
          <div class="flex min-w-0 flex-1 flex-col">
            <p
              class="truncate font-medium text-surface-800 dark:text-surface-100"
            >
              {{ p.name }}
            </p>
            <p class="truncate text-xs text-surface-500 dark:text-surface-400">
              {{ p.kind }} · {{ p.defaultModel
              }}{{ p.hasKey ? "" : " · no key" }}
            </p>
          </div>
          <Button
            text
            rounded
            severity="secondary"
            aria-label="Edit provider"
            @click="openEdit(p)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="16" />
          </Button>
          <Button
            text
            rounded
            severity="danger"
            aria-label="Delete provider"
            @click="remove(p)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="16" />
          </Button>
        </li>
      </ul>

      <div v-if="providers.length" class="flex">
        <Button :pt="{ root: btnGhost }" @click="openCreate">
          <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="16" />
          Add provider
        </Button>
      </div>
    </template>

    <Dialog
      :visible="dialogOpen"
      modal
      :header="isEdit ? 'Edit provider' : 'Add provider'"
      :closable="!busy"
      :pt="{
        root: dialogRoot('max-w-lg'),
        content: 'min-h-0 max-h-[70vh] overflow-auto p-5',
      }"
      @update:visible="dialogOpen = $event"
    >
      <div class="flex min-w-0 flex-col gap-4">
        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Provider type
          </label>
          <Select
            :model-value="form.kind"
            :options="kindOptions"
            option-label="label"
            option-value="value"
            @update:model-value="form.kind = $event"
          />
        </div>

        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            for="ai-name"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Name <span class="text-red-500">*</span>
          </label>
          <InputText
            id="ai-name"
            :model-value="form.name"
            placeholder="e.g. My OpenAI"
            @update:model-value="form.name = $event ?? ''"
          />
        </div>

        <div v-if="needsBaseUrl" class="flex min-w-0 flex-col gap-1.5">
          <label
            for="ai-base-url"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Base URL <span class="text-red-500">*</span>
          </label>
          <InputText
            id="ai-base-url"
            :model-value="form.baseUrl"
            placeholder="https://host/v1"
            @update:model-value="form.baseUrl = $event ?? ''"
          />
        </div>

        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            for="ai-key"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            API key
            <span v-if="!isEdit && !needsBaseUrl" class="text-red-500">*</span>
          </label>
          <Password
            input-id="ai-key"
            :model-value="form.apiKey"
            :feedback="false"
            toggle-mask
            :placeholder="keyPlaceholder"
            fluid
            @update:model-value="form.apiKey = $event ?? ''"
          />
        </div>

        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            for="ai-models"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Models
          </label>
          <InputText
            id="ai-models"
            :model-value="form.models"
            placeholder="gpt-4o, gpt-4o-mini"
            @update:model-value="form.models = $event ?? ''"
          />
          <p class="text-xs text-surface-400">
            Comma-separated allow-list. Leave blank for provider defaults.
          </p>
        </div>

        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            for="ai-default"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Default model <span class="text-red-500">*</span>
          </label>
          <InputText
            id="ai-default"
            :model-value="form.defaultModel"
            placeholder="gpt-4o"
            @update:model-value="form.defaultModel = $event ?? ''"
          />
        </div>

        <p v-if="formError" class="text-sm text-red-500">{{ formError }}</p>
      </div>

      <template #footer>
        <Button
          :pt="{ root: btnGhost }"
          :disabled="busy"
          @click="dialogOpen = false"
        >
          Cancel
        </Button>
        <Button :pt="{ root: btnPrimary }" :loading="busy" @click="save">
          {{ isEdit ? "Save" : "Add provider" }}
        </Button>
      </template>
    </Dialog>
  </div>
</template>
