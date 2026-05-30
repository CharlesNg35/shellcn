<script setup lang="ts">
import Breadcrumb from "primevue/breadcrumb";
import type { RouteLocationRaw } from "vue-router";
import AppIcon from "./AppIcon.vue";

export interface Crumb {
  label: string;
  to?: RouteLocationRaw;
}

defineProps<{ items: Crumb[] }>();

const linkClass =
  "rounded px-1.5 py-0.5 text-surface-500 transition-colors hover:text-surface-800 focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none dark:hover:text-surface-200";
const currentClass =
  "px-1.5 py-0.5 font-medium text-surface-800 dark:text-surface-100";
</script>

<template>
  <Breadcrumb :model="items" aria-label="Breadcrumb">
    <template #item="{ item }">
      <RouterLink
        v-if="(item as Crumb).to"
        :to="(item as Crumb).to!"
        :class="linkClass"
      >
        {{ item.label }}
      </RouterLink>
      <span v-else aria-current="page" :class="currentClass">
        {{ item.label }}
      </span>
    </template>
    <template #separator>
      <AppIcon :icon="{ type: 'lucide', value: 'chevron-right' }" :size="14" />
    </template>
  </Breadcrumb>
</template>
