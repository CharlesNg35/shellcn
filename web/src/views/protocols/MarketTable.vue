<script setup lang="ts">
import { computed, ref } from "vue";
import IconField from "primevue/iconfield";
import InputIcon from "primevue/inputicon";
import InputText from "primevue/inputtext";
import Tag from "primevue/tag";
import AppIcon from "../../components/AppIcon.vue";
import type { MarketEntry } from "../../types/projection";
import MarketPluginRow from "./MarketPluginRow.vue";

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
      ...(entry.latest?.platforms ?? []),
      entry.installedVersion,
      ...entry.maintainers,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase()
      .includes(q),
  );
});

const installedCount = computed(
  () => props.entries.filter((entry) => entry.managed).length,
);

const updateCount = computed(
  () => props.entries.filter((entry) => entry.updateAvailable).length,
);
</script>

<template>
  <div class="flex flex-col gap-4">
    <div
      class="flex flex-col gap-3 rounded-lg border border-surface-200 bg-surface-0 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-surface-800 dark:bg-surface-950"
    >
      <div class="min-w-0">
        <p class="text-sm font-medium text-surface-800 dark:text-surface-100">
          Marketplace
        </p>
        <p class="text-xs text-surface-500 dark:text-surface-400">
          Install plugins from the registry into this gateway.
        </p>
      </div>
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div class="flex flex-wrap gap-1.5">
          <Tag
            :value="`${props.entries.length} available`"
            severity="secondary"
          />
          <Tag
            v-if="installedCount"
            :value="`${installedCount} installed`"
            severity="success"
          />
          <Tag
            v-if="updateCount"
            :value="`${updateCount} updates`"
            severity="warn"
          />
        </div>
        <IconField class="relative w-full sm:w-72">
          <InputText
            v-model="query"
            class="w-full pr-9"
            placeholder="Search plugins"
            aria-label="Search marketplace plugins"
          />
          <InputIcon
            class="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-surface-400"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="14" />
          </InputIcon>
        </IconField>
      </div>
    </div>

    <div v-if="props.loading" class="flex flex-col gap-3">
      <div
        v-for="i in 6"
        :key="i"
        class="grid animate-pulse gap-3 rounded-lg border border-surface-200 bg-surface-0 p-4 lg:grid-cols-[minmax(0,1fr)_10rem] lg:items-center dark:border-surface-800 dark:bg-surface-950"
      >
        <div class="flex gap-3">
          <div
            class="h-10 w-10 shrink-0 rounded-md bg-surface-100 dark:bg-surface-800"
          />
          <div class="min-w-0 flex-1 space-y-2">
            <div class="h-4 w-44 rounded bg-surface-100 dark:bg-surface-800" />
            <div
              class="h-3 w-full rounded bg-surface-100 dark:bg-surface-800"
            />
            <div class="h-3 w-2/3 rounded bg-surface-100 dark:bg-surface-800" />
          </div>
        </div>
        <div class="h-8 rounded bg-surface-100 dark:bg-surface-800" />
      </div>
    </div>

    <div v-else-if="filteredEntries.length" class="flex flex-col gap-3">
      <MarketPluginRow
        v-for="entry in filteredEntries"
        :key="entry.name"
        :entry="entry"
        :installing="props.installing[entry.name] ?? false"
        :uninstalling="props.uninstalling[entry.name] ?? false"
        @install="emit('install', $event)"
        @uninstall="emit('uninstall', $event)"
      />
    </div>

    <div
      v-else
      class="flex flex-col items-center gap-2 rounded-lg border border-dashed border-surface-200 bg-surface-0 py-12 text-center dark:border-surface-800 dark:bg-surface-950"
    >
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
  </div>
</template>
