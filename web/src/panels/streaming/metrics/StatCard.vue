<script setup lang="ts">
defineProps<{ label: string; value: number | null; unit?: string }>();
</script>

<template>
  <div
    class="rounded-xl border border-surface-200 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900"
  >
    <p class="text-xs tracking-wide text-surface-400 uppercase">{{ label }}</p>
    <p class="mt-1 text-2xl font-semibold text-surface-900 dark:text-surface-0">
      <template v-if="value === null">—</template>
      <template v-else
        >{{ value
        }}<span class="ml-0.5 text-base font-normal text-surface-400">{{
          unit
        }}</span></template
      >
    </p>
  </div>
</template>
