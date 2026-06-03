<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Select from "primevue/select";
import AppIcon from "../components/AppIcon.vue";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import { adminProtocolsApi } from "../api/admin";
import { useNotify } from "../composables/useNotify";
import type {
  ProtocolAdminItem,
  ProtocolAvailability,
} from "../types/projection";

const notify = useNotify();

const crumbs = [
  { label: "Settings", to: { name: "settings" } },
  { label: "Protocols" },
];

const availabilityChoices: { label: string; value: ProtocolAvailability }[] = [
  { label: "Enabled", value: "enabled" },
  { label: "Admins only", value: "admin_only" },
  { label: "Disabled", value: "disabled" },
];

const tab = ref("builtin");
const protocols = ref<ProtocolAdminItem[]>([]);
const loading = ref(true);
const saving = ref<Record<string, boolean>>({});

const builtIn = computed(() => protocols.value.filter((p) => !p.external));
const external = computed(() => protocols.value.filter((p) => p.external));

async function load(): Promise<void> {
  loading.value = true;
  try {
    protocols.value = await adminProtocolsApi.list();
  } finally {
    loading.value = false;
  }
}
onMounted(load);

function transportLabel(p: ProtocolAdminItem): string {
  if (!p.transports?.length) return "—";
  return p.transports
    .map((t) => (t === "agent" ? "Agent" : "Direct"))
    .join(", ");
}

async function setAvailability(
  item: ProtocolAdminItem,
  next: ProtocolAvailability,
): Promise<void> {
  const previous = item.availability;
  if (next === previous) return;
  saving.value = { ...saving.value, [item.name]: true };
  item.availability = next;
  try {
    await adminProtocolsApi.setAvailability(item.name, next);
    notify.success("Protocol updated", item.title);
  } catch {
    item.availability = previous;
    notify.error("Could not update protocol", item.title);
  } finally {
    saving.value = { ...saving.value, [item.name]: false };
  }
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <AppBreadcrumb :items="crumbs" />

    <div class="flex flex-col gap-1">
      <h1 class="text-xl font-semibold text-surface-900 dark:text-surface-0">
        Protocols
      </h1>
      <p class="text-sm text-surface-500 dark:text-surface-400">
        Control which protocols users can connect with. Admins-only protocols
        are hidden from everyone else; disabled protocols are hidden from all
        and cannot open a session.
      </p>
    </div>

    <Tabs
      :value="tab"
      :pt="{ root: 'flex min-h-0 flex-1 flex-col' }"
      @update:value="tab = String($event)"
    >
      <TabList>
        <Tab value="builtin">
          <AppIcon :icon="{ type: 'lucide', value: 'box' }" :size="14" />
          Built-in
          <span class="text-surface-400">({{ builtIn.length }})</span>
        </Tab>
        <Tab value="external">
          <AppIcon :icon="{ type: 'lucide', value: 'puzzle' }" :size="14" />
          External
          <span class="text-surface-400">({{ external.length }})</span>
        </Tab>
      </TabList>
      <TabPanels>
        <TabPanel value="builtin" class="flex h-full flex-col">
          <div class="min-h-0 flex-1">
            <DataTable
              :value="builtIn"
              :loading="loading"
              scrollable
              scroll-height="flex"
            >
              <Column header="Protocol">
                <template #body="{ data }">
                  <span class="flex items-center gap-2">
                    <AppIcon
                      :icon="(data as ProtocolAdminItem).icon"
                      :size="18"
                    />
                    <span class="min-w-0">
                      <span
                        class="block font-medium text-surface-800 dark:text-surface-100"
                        >{{ (data as ProtocolAdminItem).title }}</span
                      >
                      <span class="block text-xs text-surface-400">{{
                        (data as ProtocolAdminItem).name
                      }}</span>
                    </span>
                  </span>
                </template>
              </Column>
              <Column header="Transports">
                <template #body="{ data }">
                  <span class="text-sm text-surface-500">{{
                    transportLabel(data as ProtocolAdminItem)
                  }}</span>
                </template>
              </Column>
              <Column header="Capabilities">
                <template #body="{ data }">
                  <div class="flex flex-wrap items-center gap-1">
                    <span
                      v-for="risk in (data as ProtocolAdminItem).risks"
                      :key="risk"
                      class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-600 capitalize dark:bg-surface-800 dark:text-surface-300"
                      >{{ risk }}</span
                    >
                    <span
                      v-if="(data as ProtocolAdminItem).recording?.length"
                      class="inline-flex items-center gap-1 text-xs text-surface-400"
                    >
                      <AppIcon
                        :icon="{ type: 'lucide', value: 'video' }"
                        :size="12"
                      />
                      {{ (data as ProtocolAdminItem).recording!.join(", ") }}
                    </span>
                    <span
                      v-if="
                        !(data as ProtocolAdminItem).risks?.length &&
                        !(data as ProtocolAdminItem).recording?.length
                      "
                      class="text-sm text-surface-400"
                      >—</span
                    >
                  </div>
                </template>
              </Column>
              <Column header="Availability" :pt="{ bodyCell: 'w-44' }">
                <template #body="{ data }">
                  <Select
                    :model-value="(data as ProtocolAdminItem).availability"
                    :options="availabilityChoices"
                    option-label="label"
                    option-value="value"
                    :disabled="saving[(data as ProtocolAdminItem).name]"
                    :aria-label="`Availability for ${(data as ProtocolAdminItem).title}`"
                    fluid
                    @update:model-value="
                      setAvailability(data as ProtocolAdminItem, $event)
                    "
                  />
                </template>
              </Column>
              <template #empty>No built-in protocols.</template>
            </DataTable>
          </div>
        </TabPanel>

        <TabPanel value="external" class="flex h-full flex-col">
          <div
            v-if="!loading && !external.length"
            class="flex flex-1 flex-col items-center justify-center gap-2 py-12 text-center"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'puzzle' }"
              :size="28"
              class="text-surface-300"
            />
            <p class="font-medium text-surface-700 dark:text-surface-200">
              No external protocols loaded
            </p>
            <p class="max-w-sm text-sm text-surface-500 dark:text-surface-400">
              Drop a compiled plugin binary into the server's
              <code
                class="rounded bg-surface-100 px-1 py-0.5 dark:bg-surface-800"
                >plugins.d/</code
              >
              directory and restart to load it.
            </p>
          </div>
          <div v-else class="min-h-0 flex-1">
            <DataTable
              :value="external"
              :loading="loading"
              scrollable
              scroll-height="flex"
            >
              <Column header="Protocol">
                <template #body="{ data }">
                  <span class="flex items-center gap-2">
                    <AppIcon
                      :icon="(data as ProtocolAdminItem).icon"
                      :size="18"
                    />
                    <span class="min-w-0">
                      <span
                        class="block font-medium text-surface-800 dark:text-surface-100"
                        >{{ (data as ProtocolAdminItem).title }}</span
                      >
                      <span class="block text-xs text-surface-400">{{
                        (data as ProtocolAdminItem).name
                      }}</span>
                    </span>
                  </span>
                </template>
              </Column>
              <Column field="version" header="Version">
                <template #body="{ data }">
                  <span class="text-sm text-surface-500">{{
                    (data as ProtocolAdminItem).version || "—"
                  }}</span>
                </template>
              </Column>
              <Column header="Status">
                <template #body="{ data }">
                  <span
                    class="inline-flex items-center gap-1.5 text-sm"
                    :class="
                      (data as ProtocolAdminItem).healthy
                        ? 'text-emerald-600'
                        : 'text-rose-600'
                    "
                  >
                    <span
                      class="h-2 w-2 rounded-full"
                      :class="
                        (data as ProtocolAdminItem).healthy
                          ? 'bg-emerald-500'
                          : 'bg-rose-500'
                      "
                    />
                    {{
                      (data as ProtocolAdminItem).healthy
                        ? "Running"
                        : "Offline"
                    }}
                  </span>
                </template>
              </Column>
              <Column header="Capabilities">
                <template #body="{ data }">
                  <div class="flex flex-wrap items-center gap-1">
                    <span
                      v-for="risk in (data as ProtocolAdminItem).risks"
                      :key="risk"
                      class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-600 capitalize dark:bg-surface-800 dark:text-surface-300"
                      >{{ risk }}</span
                    >
                    <span
                      v-if="(data as ProtocolAdminItem).recording?.length"
                      class="inline-flex items-center gap-1 text-xs text-surface-400"
                    >
                      <AppIcon
                        :icon="{ type: 'lucide', value: 'video' }"
                        :size="12"
                      />
                      {{ (data as ProtocolAdminItem).recording!.join(", ") }}
                    </span>
                    <span
                      v-if="
                        !(data as ProtocolAdminItem).risks?.length &&
                        !(data as ProtocolAdminItem).recording?.length
                      "
                      class="text-sm text-surface-400"
                      >—</span
                    >
                  </div>
                </template>
              </Column>
              <Column header="Availability" :pt="{ bodyCell: 'w-44' }">
                <template #body="{ data }">
                  <Select
                    :model-value="(data as ProtocolAdminItem).availability"
                    :options="availabilityChoices"
                    option-label="label"
                    option-value="value"
                    :disabled="saving[(data as ProtocolAdminItem).name]"
                    :aria-label="`Availability for ${(data as ProtocolAdminItem).title}`"
                    fluid
                    @update:model-value="
                      setAvailability(data as ProtocolAdminItem, $event)
                    "
                  />
                </template>
              </Column>
              <template #empty>No external protocols.</template>
            </DataTable>
          </div>
        </TabPanel>
      </TabPanels>
    </Tabs>
  </div>
</template>
