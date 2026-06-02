<script setup lang="ts">
import ProgressSpinner from "primevue/progressspinner";

withDefaults(defineProps<{ label?: string }>(), { label: "Loading" });
</script>

<template>
  <div
    class="flex h-full min-h-0 flex-col items-center justify-center gap-3 p-4 text-surface-400"
    role="status"
    aria-live="polite"
    data-test="panel-loader"
  >
    <ProgressSpinner :aria-label="label" />
    <span class="text-xs font-medium">{{ label }}</span>
  </div>
</template>
