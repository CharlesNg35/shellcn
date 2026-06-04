<script setup lang="ts">
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";
import type { MarketEntry } from "../../types/projection";

const props = defineProps<{
  entries: MarketEntry[];
  loading: boolean;
  installing: Record<string, boolean>;
}>();

const emit = defineEmits<{
  (e: "install", entry: MarketEntry): void;
}>();

function action(entry: MarketEntry): string | null {
  if (!entry.compatible) return null;
  if (!entry.managed) return "Install";
  if (entry.updateAvailable) return "Update";
  return null;
}
</script>

<template>
  <DataTable
    :value="props.entries"
    :loading="props.loading"
    scrollable
    scroll-height="flex"
  >
    <Column header="Plugin">
      <template #body="{ data }">
        <span class="flex items-center gap-2">
          <AppIcon
            v-if="(data as MarketEntry).latest"
            :icon="(data as MarketEntry).latest!.icon"
            :size="18"
          />
          <span class="min-w-0">
            <span
              class="block font-medium text-surface-800 dark:text-surface-100"
              >{{ (data as MarketEntry).displayName }}</span
            >
            <span class="block text-xs text-surface-400">
              {{ (data as MarketEntry).name }} ·
              {{ (data as MarketEntry).license }}
            </span>
          </span>
        </span>
      </template>
    </Column>
    <Column header="Description">
      <template #body="{ data }">
        <span class="block max-w-md text-sm text-surface-500">
          {{ (data as MarketEntry).description }}
        </span>
        <a
          :href="`https://${(data as MarketEntry).repo}`"
          target="_blank"
          rel="noopener noreferrer"
          class="text-xs text-primary-500 hover:underline"
          >{{ (data as MarketEntry).repo }}</a
        >
      </template>
    </Column>
    <Column header="Latest" :pt="{ bodyCell: 'w-28' }">
      <template #body="{ data }">
        <span class="text-sm text-surface-500">
          {{ (data as MarketEntry).latest?.version ?? "—" }}
        </span>
      </template>
    </Column>
    <Column header="Status" :pt="{ bodyCell: 'w-44' }">
      <template #body="{ data }">
        <div class="flex items-center gap-2">
          <Button
            v-if="action(data as MarketEntry)"
            :label="action(data as MarketEntry)!"
            size="small"
            :loading="props.installing[(data as MarketEntry).name]"
            :aria-label="`${action(data as MarketEntry)} ${(data as MarketEntry).displayName}`"
            @click="emit('install', data as MarketEntry)"
          />
          <span
            v-else-if="(data as MarketEntry).managed"
            class="inline-flex items-center gap-1.5 text-sm text-emerald-600"
          >
            <span class="h-2 w-2 rounded-full bg-emerald-500" />
            Installed
            {{
              (data as MarketEntry).installedVersion
                ? `v${(data as MarketEntry).installedVersion}`
                : ""
            }}
          </span>
          <span v-else class="text-sm text-surface-400">Incompatible</span>
        </div>
      </template>
    </Column>
    <template #empty>No plugins in the registry yet.</template>
  </DataTable>
</template>
