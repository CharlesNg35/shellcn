<script setup lang="ts">
defineProps<{ panel: string }>();
</script>

<template>
  <div
    class="flex h-full flex-col items-center justify-center gap-2 p-6 text-center"
  >
    <p class="text-surface-600 dark:text-surface-300">
      No renderer for panel type
      <code
        class="rounded bg-surface-100 px-1.5 py-0.5 text-sm dark:bg-surface-800"
        >{{ panel }}</code
      >
    </p>
    <p class="max-w-sm text-sm text-surface-400">
      This panel type isn't available yet. It lands with the plugin that needs
      it.
    </p>
  </div>
</template>
