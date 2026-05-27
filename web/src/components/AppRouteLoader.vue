<script setup lang="ts">
withDefaults(
  defineProps<{
    label?: string;
    fixed?: boolean;
  }>(),
  {
    label: "Loading",
    fixed: true,
  },
);
</script>

<template>
  <div
    :class="
      fixed
        ? 'fixed inset-0 z-50 bg-surface-50/60 backdrop-blur-[2px] dark:bg-surface-950/60'
        : 'h-full bg-surface-50 dark:bg-surface-950'
    "
    class="flex flex-col items-center justify-center gap-4.5 text-surface-500 dark:text-surface-400"
    role="status"
    :aria-label="label"
  >
    <span
      class="h-8 w-8 animate-spin rounded-full border-3 border-surface-200 border-t-primary-500 dark:border-surface-800 dark:border-t-primary-500"
      aria-hidden="true"
    />
    <span class="sr-only">{{ label }}</span>
  </div>
</template>
