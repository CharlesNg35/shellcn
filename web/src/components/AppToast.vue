<script setup lang="ts">
import Toast from "primevue/toast";

// Custom container keeps full styling control while reusing PrimeVue's
// positioning, lifecycle, grouping and accessibility.
const tone: Record<string, string> = {
  success:
    "border-emerald-500/40 bg-emerald-50 text-emerald-800 dark:bg-emerald-950/70 dark:text-emerald-200",
  error:
    "border-red-500/40 bg-red-50 text-red-800 dark:bg-red-950/70 dark:text-red-200",
  info: "border-surface-300 bg-surface-0 text-surface-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-200",
};
</script>

<template>
  <Toast
    position="bottom-right"
    :pt="{ root: 'fixed bottom-4 right-4 z-[100] flex w-80 flex-col gap-2' }"
  >
    <template #container="{ message, closeCallback }">
      <div
        class="flex items-start justify-between gap-3 rounded-md border px-3 py-2 text-sm shadow-lg"
        :class="tone[message.severity] ?? tone.info"
      >
        <div class="min-w-0">
          <p v-if="message.summary" class="font-medium">
            {{ message.summary }}
          </p>
          <p v-if="message.detail" class="text-current/80">
            {{ message.detail }}
          </p>
        </div>
        <button
          type="button"
          class="shrink-0 text-current/60 hover:text-current"
          aria-label="Dismiss"
          @click="closeCallback"
        >
          ✕
        </button>
      </div>
    </template>
  </Toast>
</template>
