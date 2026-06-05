<script setup lang="ts">
import { computed, ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import IconField from "primevue/iconfield";
import InputIcon from "primevue/inputicon";
import InputText from "primevue/inputtext";
import Tag from "primevue/tag";
import AppIcon from "../../components/AppIcon.vue";
import type { MarketEntry } from "../../types/projection";

const props = defineProps<{
  entries: MarketEntry[];
  loading: boolean;
  installing: Record<string, boolean>;
  uninstalling: Record<string, boolean>;
}>();

const emit = defineEmits<{
  (e: "install", entry: MarketEntry): void;
  (e: "uninstall", entry: MarketEntry): void;
}>();

const query = ref("");

const filteredEntries = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return props.entries;
  return props.entries.filter((entry) =>
    [
      entry.displayName,
      entry.name,
      entry.description,
      entry.repo,
      entry.homepage,
      entry.license,
      entry.latest?.version,
      entry.installedVersion,
      ...entry.maintainers,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase()
      .includes(q),
  );
});

function action(entry: MarketEntry): string | null {
  if (!entry.compatible) return null;
  if (!entry.managed) return "Install";
  if (entry.updateAvailable) return "Update";
  return null;
}

function status(entry: MarketEntry): {
  value: string;
  severity: "success" | "info" | "warn" | "secondary";
} {
  if (!entry.compatible) return { value: "Unavailable", severity: "secondary" };
  if (!entry.managed) return { value: "Not installed", severity: "secondary" };
  if (entry.updateAvailable)
    return { value: "Update available", severity: "warn" };
  return { value: "Installed", severity: "success" };
}

function version(value?: string): string {
  return value ? `v${value}` : "Not installed";
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-3">
    <div
      class="flex flex-col gap-3 rounded-lg border border-surface-200 bg-surface-0 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-surface-800 dark:bg-surface-950"
    >
      <div>
        <p class="text-sm font-medium text-surface-800 dark:text-surface-100">
          Marketplace plugins
        </p>
        <p class="text-xs text-surface-500 dark:text-surface-400">
          ShellCN-maintained external plugins install into this gateway.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <IconField class="w-full sm:w-72">
          <InputIcon class="pi pi-search" />
          <InputText
            v-model="query"
            class="w-full"
            placeholder="Search plugins"
            aria-label="Search marketplace plugins"
          />
        </IconField>
        <span
          class="hidden text-xs whitespace-nowrap text-surface-400 sm:inline"
        >
          {{ filteredEntries.length }} of {{ props.entries.length }}
        </span>
      </div>
    </div>

    <div
      class="min-h-0 flex-1 overflow-hidden rounded-lg border border-surface-200 bg-surface-0 dark:border-surface-800 dark:bg-surface-950"
    >
      <DataTable
        :value="filteredEntries"
        :loading="props.loading"
        scrollable
        scroll-height="flex"
        data-key="name"
        :pt="{ root: 'h-full', table: 'min-w-[760px]' }"
      >
        <Column header="Plugin">
          <template #body="{ data }">
            <span class="flex min-w-0 items-center gap-3">
              <span
                class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-300"
              >
                <AppIcon
                  v-if="(data as MarketEntry).latest"
                  :icon="(data as MarketEntry).latest!.icon"
                  :size="19"
                />
                <AppIcon
                  v-else
                  :icon="{ type: 'lucide', value: 'puzzle' }"
                  :size="18"
                />
              </span>
              <span class="min-w-0">
                <span
                  class="block truncate font-medium text-surface-800 dark:text-surface-100"
                  >{{ (data as MarketEntry).displayName }}</span
                >
                <span class="block truncate text-xs text-surface-400">
                  {{ (data as MarketEntry).name }} ·
                  {{ (data as MarketEntry).license }}
                </span>
              </span>
            </span>
          </template>
        </Column>
        <Column header="Details">
          <template #body="{ data }">
            <span
              class="block max-w-md text-sm text-surface-600 dark:text-surface-300"
            >
              {{ (data as MarketEntry).description }}
            </span>
            <a
              :href="`https://${(data as MarketEntry).repo}`"
              target="_blank"
              rel="noopener noreferrer"
              class="mt-1 inline-block text-xs text-primary-500 hover:underline"
              >{{ (data as MarketEntry).repo }}</a
            >
          </template>
        </Column>
        <Column header="Version" :pt="{ bodyCell: 'w-36' }">
          <template #body="{ data }">
            <span class="block text-sm text-surface-700 dark:text-surface-200">
              {{ version((data as MarketEntry).latest?.version) }}
            </span>
            <span
              v-if="(data as MarketEntry).installedVersion"
              class="block text-xs text-surface-400"
            >
              Installed {{ version((data as MarketEntry).installedVersion) }}
            </span>
          </template>
        </Column>
        <Column header="Status" :pt="{ bodyCell: 'w-48' }">
          <template #body="{ data }">
            <Tag
              :value="status(data as MarketEntry).value"
              :severity="status(data as MarketEntry).severity"
            />
          </template>
        </Column>
        <Column header="" :pt="{ bodyCell: 'w-56' }">
          <template #body="{ data }">
            <div class="flex justify-end gap-2">
              <Button
                v-if="action(data as MarketEntry)"
                :label="action(data as MarketEntry)!"
                size="small"
                :loading="props.installing[(data as MarketEntry).name]"
                :disabled="props.uninstalling[(data as MarketEntry).name]"
                :aria-label="`${action(data as MarketEntry)} ${(data as MarketEntry).displayName}`"
                @click="emit('install', data as MarketEntry)"
              />
              <Button
                v-if="(data as MarketEntry).managed"
                label="Uninstall"
                severity="danger"
                variant="outlined"
                size="small"
                :loading="props.uninstalling[(data as MarketEntry).name]"
                :disabled="props.installing[(data as MarketEntry).name]"
                :aria-label="`Uninstall ${(data as MarketEntry).displayName}`"
                @click="emit('uninstall', data as MarketEntry)"
              />
            </div>
          </template>
        </Column>
        <template #empty>
          <div class="flex flex-col items-center gap-2 py-10 text-center">
            <AppIcon
              :icon="{ type: 'lucide', value: 'search-x' }"
              :size="24"
              class="text-surface-300"
            />
            <p class="text-sm text-surface-500">
              {{
                query.trim()
                  ? "No marketplace plugins match your search."
                  : "No plugins in the registry yet."
              }}
            </p>
          </div>
        </template>
      </DataTable>
    </div>
  </div>
</template>
