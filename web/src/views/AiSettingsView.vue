<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import Tag from "primevue/tag";
import { ApiError } from "../api/client";
import { aiApi, type AiProviderSummary } from "../api/ai";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import AppIcon from "../components/AppIcon.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { useNotify } from "../composables/useNotify";
import { useAiProvidersStore } from "../stores/aiProviders";
import AiProviderDialog from "./ai-settings/AiProviderDialog.vue";
import AiProviderList from "./ai-settings/AiProviderList.vue";
import SharedAiPanel from "./ai-settings/SharedAiPanel.vue";

const notify = useNotify();
const { confirmDanger } = useConfirmAction();
const aiProviders = useAiProvidersStore();

const tab = ref("providers");
const dialogOpen = ref(false);
const editingProvider = ref<AiProviderSummary | null>(null);

const crumbs = [
  { label: "Settings", to: { name: "settings" } },
  { label: "AI providers" },
];

const providers = computed(() => aiProviders.providers);
const global = computed(() => aiProviders.global);
const loading = computed(() => aiProviders.loading);
const sharedConfigured = computed(() => Boolean(global.value?.configured));

function syncSharedTab(): void {
  if (!aiProviders.global?.configured && tab.value === "shared") {
    tab.value = "providers";
  }
}

function errorMessage(err: unknown): string | undefined {
  return err instanceof ApiError || err instanceof Error
    ? err.message
    : undefined;
}

async function load(): Promise<void> {
  try {
    await aiProviders.load();
    syncSharedTab();
  } catch (err) {
    notify.error("Failed to load AI settings", errorMessage(err));
  }
}

async function refreshSettings(): Promise<void> {
  try {
    await aiProviders.refresh();
    syncSharedTab();
  } catch (err) {
    notify.error("Failed to refresh AI settings", errorMessage(err));
  }
}

function openCreate(): void {
  editingProvider.value = null;
  dialogOpen.value = true;
}

function openEdit(provider: AiProviderSummary): void {
  editingProvider.value = provider;
  dialogOpen.value = true;
}

async function afterSave(): Promise<void> {
  notify.success(editingProvider.value ? "Provider updated" : "Provider added");
  await refreshSettings();
}

function remove(provider: AiProviderSummary): void {
  confirmDanger({
    header: "Delete provider",
    message: `Delete "${provider.name}"? Conversations using it will fall back to another configured provider.`,
    accept: async () => {
      try {
        await aiApi.remove(provider.id);
        notify.success("Provider deleted");
        await refreshSettings();
      } catch (err) {
        notify.error("Failed to delete", errorMessage(err));
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <AppBreadcrumb :items="crumbs" />

    <div>
      <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
        AI providers
      </h1>
      <p class="mt-1 max-w-4xl text-sm text-surface-500 dark:text-surface-400">
        Configure personal providers and view the shared workspace provider.
      </p>
    </div>

    <Tabs
      :value="tab"
      :pt="{ root: 'flex min-h-0 flex-1 flex-col' }"
      @update:value="tab = String($event)"
    >
      <TabList>
        <Tab value="providers">
          <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="14" />
          My providers
        </Tab>
        <Tab value="shared" :disabled="!sharedConfigured">
          <AppIcon :icon="{ type: 'lucide', value: 'bot' }" :size="14" />
          Shared AI
          <Tag
            :value="sharedConfigured ? 'Configured' : 'Not configured'"
            :severity="sharedConfigured ? 'success' : 'secondary'"
            class="ml-1"
          />
        </Tab>
      </TabList>
      <TabPanels class="min-h-0 flex-1">
        <TabPanel value="providers" class="min-h-0">
          <AiProviderList
            :providers="providers"
            :loading="loading"
            @add="openCreate"
            @edit="openEdit"
            @remove="remove"
          />
        </TabPanel>
        <TabPanel value="shared" class="min-h-0">
          <SharedAiPanel :global="global" />
        </TabPanel>
      </TabPanels>
    </Tabs>

    <AiProviderDialog
      v-model:visible="dialogOpen"
      :providers="providers"
      :provider="editingProvider"
      @saved="afterSave"
    />
  </div>
</template>
