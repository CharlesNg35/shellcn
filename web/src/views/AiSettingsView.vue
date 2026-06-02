<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import Tag from "primevue/tag";
import { ApiError } from "../api/client";
import { aiApi, type AiGlobalStatus, type AiProviderSummary } from "../api/ai";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import AppIcon from "../components/AppIcon.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { useNotify } from "../composables/useNotify";
import AiProviderDialog from "./ai-settings/AiProviderDialog.vue";
import AiProviderList from "./ai-settings/AiProviderList.vue";
import SharedAiPanel from "./ai-settings/SharedAiPanel.vue";

const notify = useNotify();
const { confirmDanger } = useConfirmAction();

const loading = ref(true);
const providers = ref<AiProviderSummary[]>([]);
const global = ref<AiGlobalStatus | null>(null);
const tab = ref("providers");
const dialogOpen = ref(false);
const editingProvider = ref<AiProviderSummary | null>(null);

const crumbs = [
  { label: "Settings", to: { name: "settings" } },
  { label: "AI providers" },
];

const sharedConfigured = computed(() => Boolean(global.value?.configured));

async function load(): Promise<void> {
  loading.value = true;
  try {
    const [g, list] = await Promise.all([aiApi.global(), aiApi.list()]);
    global.value = g;
    providers.value = list;
    if (!g.configured && tab.value === "shared") {
      tab.value = "providers";
    }
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
  editingProvider.value = null;
  dialogOpen.value = true;
}

function openEdit(provider: AiProviderSummary): void {
  editingProvider.value = provider;
  dialogOpen.value = true;
}

async function afterSave(): Promise<void> {
  notify.success(editingProvider.value ? "Provider updated" : "Provider added");
  await load();
}

function remove(provider: AiProviderSummary): void {
  confirmDanger({
    header: "Delete provider",
    message: `Delete "${provider.name}"? Conversations using it will fall back to another configured provider.`,
    accept: async () => {
      try {
        await aiApi.remove(provider.id);
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
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <AppBreadcrumb :items="crumbs" />

    <div>
      <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
        AI providers
      </h1>
      <p class="mt-1 max-w-2xl text-sm text-surface-500 dark:text-surface-400">
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
