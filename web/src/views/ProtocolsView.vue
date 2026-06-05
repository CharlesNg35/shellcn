<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import AppIcon from "../components/AppIcon.vue";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import ProtocolTable from "./protocols/ProtocolTable.vue";
import MarketTable from "./protocols/MarketTable.vue";
import { useProtocolsAdmin } from "../composables/useProtocolsAdmin";
import { useMarketAdmin } from "../composables/useMarketAdmin";
import { useConfirmAction } from "../composables/useConfirmAction";
import { useConnectionsStore } from "../stores/connections";
import type { MarketEntry } from "../types/projection";

const crumbs = [
  { label: "Settings", to: { name: "settings" } },
  { label: "Protocols" },
];

type ProtocolsTab = "builtin" | "external" | "market";

const route = useRoute();
const router = useRouter();

function normalizeTab(value: unknown): ProtocolsTab {
  const tabValue = Array.isArray(value) ? value[0] : value;
  return tabValue === "external" || tabValue === "market"
    ? tabValue
    : "builtin";
}

const tab = ref<ProtocolsTab>(normalizeTab(route.query.tab));
const conns = useConnectionsStore();
const { confirmDanger } = useConfirmAction();

const {
  pluginsDir,
  loading,
  saving,
  builtIn,
  external,
  load,
  setAvailability,
} = useProtocolsAdmin();

async function refreshAfterMarketChange(): Promise<void> {
  await Promise.all([load(), conns.refreshPlugins()]);
}

const {
  enabled: marketEnabled,
  entries: marketEntries,
  loading: marketLoading,
  installing,
  uninstalling,
  load: loadMarket,
  install,
  uninstall,
} = useMarketAdmin(refreshAfterMarketChange);

function confirmUninstall(entry: MarketEntry): void {
  confirmDanger({
    header: "Uninstall plugin",
    message: `Uninstall ${entry.displayName}? Existing connections that use this protocol will stop working until it is installed again.`,
    acceptLabel: "Uninstall",
    accept: () => uninstall(entry),
  });
}

function setTab(value: string): void {
  const next = normalizeTab(value);
  tab.value = next;
  const query = { ...route.query };
  if (next === "builtin") {
    delete query.tab;
  } else {
    query.tab = next;
  }
  void router.replace({ query });
}

watch(
  () => route.query.tab,
  (value) => {
    tab.value = normalizeTab(value);
  },
);

onMounted(() => {
  void load();
  void loadMarket();
});
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="mx-auto flex max-w-6xl flex-col gap-5 p-8">
      <AppBreadcrumb :items="crumbs" />

      <div class="flex flex-col gap-1">
        <h1 class="text-xl font-semibold text-surface-900 dark:text-surface-0">
          Protocols
        </h1>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          Control built-in protocols, installed plugins, and Marketplace
          installs from one place. Admins-only protocols are hidden from
          everyone else; disabled protocols cannot open a session.
        </p>
      </div>

      <Tabs
        :value="tab"
        :pt="{
          root: 'flex flex-col',
          tabpanels: { root: 'overflow-visible pt-4' },
          tabpanel: { root: 'focus-visible:outline-none' },
        }"
        @update:value="setTab(String($event))"
      >
        <TabList>
          <Tab value="builtin">
            <AppIcon :icon="{ type: 'lucide', value: 'box' }" :size="14" />
            Built-in
            <span class="text-surface-400">({{ builtIn.length }})</span>
          </Tab>
          <Tab value="external">
            <AppIcon :icon="{ type: 'lucide', value: 'puzzle' }" :size="14" />
            Installed plugins
            <span class="text-surface-400">({{ external.length }})</span>
          </Tab>
          <Tab value="market">
            <AppIcon :icon="{ type: 'lucide', value: 'store' }" :size="14" />
            Marketplace
            <span class="text-surface-400">({{ marketEntries.length }})</span>
          </Tab>
        </TabList>
        <TabPanels>
          <TabPanel value="builtin">
            <div class="min-h-112">
              <ProtocolTable
                :protocols="builtIn"
                :loading="loading"
                :saving="saving"
                empty-text="No built-in protocols."
                @set-availability="setAvailability"
              />
            </div>
          </TabPanel>

          <TabPanel value="external">
            <div
              v-if="!loading && !external.length"
              class="flex min-h-112 flex-col items-center justify-center gap-2 py-12 text-center"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'puzzle' }"
                :size="28"
                class="text-surface-300"
              />
              <p class="font-medium text-surface-700 dark:text-surface-200">
                No plugin protocols installed
              </p>
              <p
                v-if="pluginsDir"
                class="max-w-sm text-sm text-surface-500 dark:text-surface-400"
              >
                Install a plugin from the Marketplace, or drop a private plugin
                binary into
                <code
                  class="rounded bg-surface-100 px-1 py-0.5 dark:bg-surface-800"
                  >{{ pluginsDir }}</code
                >
                on the server and restart the gateway.
              </p>
              <p
                v-else
                class="max-w-sm text-sm text-surface-500 dark:text-surface-400"
              >
                Plugin loading is disabled on this server.
              </p>
            </div>
            <div v-else class="min-h-112">
              <ProtocolTable
                :protocols="external"
                :loading="loading"
                :saving="saving"
                show-status
                empty-text="No plugin protocols installed."
                @set-availability="setAvailability"
              />
            </div>
          </TabPanel>

          <TabPanel value="market">
            <div
              v-if="!marketLoading && !marketEnabled"
              class="flex min-h-112 flex-col items-center justify-center gap-2 py-12 text-center"
            >
              <AppIcon
                :icon="{ type: 'lucide', value: 'store' }"
                :size="28"
                class="text-surface-300"
              />
              <p class="font-medium text-surface-700 dark:text-surface-200">
                Marketplace unavailable
              </p>
              <p
                class="max-w-sm text-sm text-surface-500 dark:text-surface-400"
              >
                The plugin registry is disabled or unreachable. Configure
                <code
                  class="rounded bg-surface-100 px-1 py-0.5 dark:bg-surface-800"
                  >plugins.market</code
                >
                on the server to enable it.
              </p>
            </div>
            <div v-else>
              <MarketTable
                :entries="marketEntries"
                :loading="marketLoading"
                :installing="installing"
                :uninstalling="uninstalling"
                @install="install"
                @uninstall="confirmUninstall"
              />
            </div>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </div>
  </div>
</template>
