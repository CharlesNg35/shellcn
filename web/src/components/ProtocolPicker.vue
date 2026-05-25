<script setup lang="ts">
import { computed, ref } from "vue";
import AppIcon from "./AppIcon.vue";
import { searchInputClass } from "../primevue/preset";
import type { PluginSummary } from "../types/projection";

const props = defineProps<{
  modelValue: string;
  plugins: PluginSummary[];
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const query = ref("");

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return props.plugins;
  return props.plugins.filter((p) =>
    [p.title, p.name, p.description ?? ""].some((f) =>
      f.toLowerCase().includes(q),
    ),
  );
});
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="relative">
      <span
        class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-surface-400"
        aria-hidden="true"
      >
        <AppIcon :icon="{ type: 'name', value: 'search' }" :size="16" />
      </span>
      <input
        v-model="query"
        type="search"
        autofocus
        placeholder="Search protocols…"
        aria-label="Search protocols"
        :class="searchInputClass"
      />
    </div>

    <div
      role="radiogroup"
      aria-label="Protocol"
      class="grid max-h-72 grid-cols-2 gap-2 overflow-auto pr-0.5"
    >
      <button
        v-for="p in filtered"
        :key="p.name"
        type="button"
        role="radio"
        :aria-checked="modelValue === p.name"
        class="group flex items-start gap-3 rounded-lg border p-3 text-left transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/60"
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
              ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/50 dark:text-primary-300'
              : 'bg-surface-100 text-surface-500 group-hover:bg-primary-50 group-hover:text-primary-600 dark:bg-surface-800 dark:text-surface-400'
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

      <p
        v-if="!filtered.length"
        class="col-span-2 py-6 text-center text-sm text-surface-400"
      >
        No protocols match “{{ query }}”.
      </p>
    </div>
  </div>
</template>
