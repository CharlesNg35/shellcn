<script setup lang="ts">
import { computed, ref } from "vue";
import IconField from "primevue/iconfield";
import InputIcon from "primevue/inputicon";
import InputText from "primevue/inputtext";
import Tag from "primevue/tag";
import AppIcon from "@/components/AppIcon.vue";
import {
  inputClass,
  searchFieldClass,
  searchIconRightClass,
} from "@/primevue/preset";
import type { MarketEntry } from "@/types/projection";
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

const sections = computed(() =>
  [
    {
      key: "installed",
      label: "Installed",
      entries: filteredEntries.value
        .filter((entry) => entry.managed)
        .sort((a, b) => Number(b.updateAvailable) - Number(a.updateAvailable)),
    },
    {
      key: "available",
      label: "Available",
      entries: filteredEntries.value.filter(
        (entry) => !entry.managed && entry.compatible,
      ),
    },
    {
      key: "unavailable",
      label: "Unavailable",
      entries: filteredEntries.value.filter(
        (entry) => !entry.managed && !entry.compatible,
      ),
    },
  ].filter((section) => section.entries.length),
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
        <IconField :class="[searchFieldClass, 'sm:w-72']">
          <InputText
            v-model="query"
            :class="[inputClass, 'h-9 pr-9']"
            placeholder="Search plugins"
            aria-label="Search marketplace plugins"
          />
          <InputIcon :class="searchIconRightClass">
            <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="14" />
          </InputIcon>
        </IconField>
      </div>
    </div>

    <div
      v-if="props.loading && !props.entries.length"
      class="flex flex-col gap-3"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="grid animate-pulse gap-3 rounded-lg border border-surface-200 bg-surface-0 p-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center dark:border-surface-800 dark:bg-surface-950"
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
        <div class="flex items-center justify-end gap-2 sm:min-w-72">
          <div class="h-8 w-9 rounded bg-surface-100 dark:bg-surface-800" />
          <div class="h-8 w-28 rounded bg-surface-100 dark:bg-surface-800" />
        </div>
      </div>
    </div>

    <div v-else-if="sections.length" class="flex flex-col gap-5">
      <section
        v-for="section in sections"
        :key="section.key"
        class="flex flex-col gap-2.5"
      >
        <div class="flex items-center gap-2 px-0.5">
          <h3
            class="text-xs font-semibold tracking-wide text-surface-500 uppercase dark:text-surface-400"
          >
            {{ section.label }}
          </h3>
          <span
            class="rounded-full bg-surface-100 px-1.5 text-xs leading-5 text-surface-500 dark:bg-surface-800 dark:text-surface-400"
          >
            {{ section.entries.length }}
          </span>
        </div>
        <TransitionGroup
          tag="div"
          class="relative flex flex-col gap-2.5"
          enter-active-class="transition duration-300 ease-out"
          enter-from-class="-translate-y-1 opacity-0"
          leave-active-class="absolute inset-x-0 transition duration-300 ease-in"
          leave-to-class="translate-y-1 opacity-0"
          move-class="transition-transform duration-300 ease-out"
        >
          <MarketPluginRow
            v-for="entry in section.entries"
            :key="entry.name"
            :entry="entry"
            :installing="props.installing[entry.name] ?? false"
            :uninstalling="props.uninstalling[entry.name] ?? false"
            @install="emit('install', $event)"
            @uninstall="emit('uninstall', $event)"
          />
        </TransitionGroup>
      </section>
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
