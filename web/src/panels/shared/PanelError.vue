<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "@/components/AppIcon.vue";

withDefaults(defineProps<{ message: string; retryable?: boolean }>(), {
  retryable: false,
});
const emit = defineEmits<{ retry: [] }>();
</script>

<template>
  <div
    role="alert"
    class="flex flex-col items-center justify-center gap-3 px-6 py-10 text-center"
  >
    <span
      class="flex h-11 w-11 items-center justify-center rounded-full bg-amber-50 text-amber-500 dark:bg-amber-950/40 dark:text-amber-400"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'triangle-alert' }" :size="20" />
    </span>
    <p class="max-w-sm text-sm text-surface-500 dark:text-surface-400">
      {{ message }}
    </p>
    <Button
      v-if="retryable"
      size="small"
      severity="secondary"
      outlined
      @click="emit('retry')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'refresh-cw' }" :size="14" />
      Retry
    </Button>
  </div>
</template>
