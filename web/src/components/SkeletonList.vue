<script setup lang="ts">
withDefaults(defineProps<{ rows?: number }>(), { rows: 6 });
</script>

<template>
  <div
    class="flex flex-col gap-2 p-4"
    aria-hidden="true"
    data-test="skeleton-list"
  >
    <div
      v-for="n in rows"
      :key="n"
      class="h-4 rounded bg-surface-200 motion-safe:animate-pulse dark:bg-surface-800"
      :style="{ width: `${60 + ((n * 13) % 35)}%` }"
    />
  </div>
</template>
