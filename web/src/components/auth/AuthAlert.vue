<script setup lang="ts">
import AppIcon from "../AppIcon.vue";

defineProps<{ message: string }>();
</script>

<template>
  <p
    class="flex items-center gap-2 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950/50 dark:text-rose-300"
    role="alert"
  >
    <AppIcon
      :icon="{ type: 'lucide', value: 'circle-alert' }"
      :size="15"
      class="shrink-0"
    />
    {{ message }}
  </p>
</template>
