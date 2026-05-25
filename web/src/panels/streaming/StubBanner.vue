<script setup lang="ts">
defineProps<{ status?: string }>();
</script>

<template>
  <div
    class="flex items-center justify-between gap-2 border-b border-amber-300/40 bg-amber-50 px-3 py-1.5 text-xs text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
  >
    <span>Stub panel — live streaming is unavailable in this preview.</span>
    <span v-if="status" class="font-mono uppercase">{{ status }}</span>
  </div>
</template>
