<script setup lang="ts">
import { computed, ref } from "vue";
import IconField from "primevue/iconfield";
import InputIcon from "primevue/inputicon";
import InputText from "primevue/inputtext";
import AppIcon from "./AppIcon.vue";
import {
  searchFieldClass,
  searchIconLeftClass,
  searchInputClass,
} from "../primevue/preset";
import type { PluginCategoryInfo, PluginSummary } from "../types/projection";

const props = defineProps<{
  modelValue: string;
  plugins: PluginSummary[];
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const query = ref("");

const fallbackCategory: PluginCategoryInfo = {
  key: "other",
  label: "Other",
  icon: { type: "lucide", value: "plug" },
  order: 1000,
};

interface PluginGroup {
  category: PluginCategoryInfo;
  plugins: PluginSummary[];
}

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return props.plugins;
  return props.plugins.filter((p) =>
    [p.title, p.name, p.description ?? "", p.category?.label ?? ""].some((f) =>
      f.toLowerCase().includes(q),
    ),
  );
});

const groups = computed<PluginGroup[]>(() => {
  const byKey = new Map<string, PluginGroup>();
  for (const plugin of filtered.value) {
    const category = plugin.category ?? fallbackCategory;
    let group = byKey.get(category.key);
    if (!group) {
      group = { category, plugins: [] };
      byKey.set(category.key, group);
    }
    group.plugins.push(plugin);
  }
  return [...byKey.values()].sort(
    (a, b) =>
      a.category.order - b.category.order ||
      a.category.label.localeCompare(b.category.label),
  );
});
</script>

<template>
  <div class="flex flex-col gap-3">
    <IconField :class="searchFieldClass">
      <InputIcon :class="searchIconLeftClass">
        <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="16" />
      </InputIcon>
      <InputText
        v-model="query"
        type="search"
        autofocus
        placeholder="Search protocols…"
        aria-label="Search protocols"
        :class="searchInputClass"
      />
    </IconField>

    <div
      role="radiogroup"
      aria-label="Protocol"
      class="max-h-72 overflow-auto pr-2 pb-2"
    >
      <section
        v-for="group in groups"
        :key="group.category.key"
        class="flex min-w-0 flex-col gap-2 pb-4 last:pb-0"
      >
        <header
          class="flex items-center gap-1.5 py-1 text-xs font-semibold tracking-wide text-surface-500 uppercase dark:text-surface-400"
        >
          <span>{{ group.category.label }}</span>
          <span class="font-normal text-surface-400">{{
            group.plugins.length
          }}</span>
        </header>

        <div class="grid grid-cols-2 gap-2">
          <button
            v-for="p in group.plugins"
            :key="p.name"
            type="button"
            role="radio"
            :aria-checked="modelValue === p.name"
            class="group flex items-start gap-3 rounded-lg border p-3 text-left transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-primary-500/60 focus-visible:outline-none"
            :class="
              modelValue === p.name
                ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-500 dark:border-primary-500 dark:bg-primary-950/30'
                : 'border-surface-200 bg-surface-0 hover:border-primary-400 hover:bg-primary-50/40 dark:border-surface-700 dark:bg-surface-950 dark:hover:border-primary-600/70 dark:hover:bg-primary-950/20'
            "
            @click="emit('update:modelValue', p.name)"
          >
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md transition-colors"
              :class="
                modelValue === p.name
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-100 dark:text-primary-700'
                  : 'bg-surface-100 text-surface-500 group-hover:bg-primary-50 group-hover:text-primary-600 dark:bg-surface-100 dark:text-surface-700 dark:group-hover:bg-primary-50 dark:group-hover:text-primary-700'
              "
            >
              <AppIcon :icon="p.icon" :size="18" />
            </span>
            <span class="flex min-w-0 flex-col">
              <span
                class="truncate text-sm font-medium text-surface-900 dark:text-surface-100"
                >{{ p.title }}</span
              >
              <span
                v-if="p.description"
                class="line-clamp-2 text-xs text-surface-500 dark:text-surface-400"
                >{{ p.description }}</span
              >
            </span>
          </button>
        </div>
      </section>

      <p
        v-if="!groups.length"
        class="col-span-2 py-6 text-center text-sm text-surface-400"
      >
        No protocols match “{{ query }}”.
      </p>
    </div>
  </div>
</template>
