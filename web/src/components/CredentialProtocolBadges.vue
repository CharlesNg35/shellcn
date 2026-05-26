<script setup lang="ts">
defineProps<{
  protocols: string[];
  labels?: Record<string, string>;
  emptyLabel?: string;
}>();
</script>

<template>
  <div
    v-if="protocols.length"
    class="flex max-w-full gap-1.5 overflow-x-auto pb-1"
  >
    <span
      v-for="protocol in protocols"
      :key="protocol"
      class="inline-flex shrink-0 items-center rounded-full border border-primary-200 bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:border-primary-800 dark:bg-primary-950 dark:text-primary-200"
    >
      {{ labels?.[protocol] ?? protocol }}
    </span>
  </div>
  <span v-else class="text-sm text-surface-400">
    {{ emptyLabel ?? "No compatible protocols" }}
  </span>
</template>
