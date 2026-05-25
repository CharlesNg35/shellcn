<script setup lang="ts">
import { useTheme } from "../composables/useTheme";

const { isDark, toggle } = useTheme();
</script>

<template>
  <div class="mx-auto max-w-2xl p-8">
    <h1
      class="mb-6 text-2xl font-semibold text-surface-900 dark:text-surface-0"
    >
      Settings
    </h1>
    <div
      class="flex items-center justify-between rounded-lg border border-surface-200 px-4 py-3 dark:border-surface-800"
    >
      <div>
        <p class="font-medium text-surface-800 dark:text-surface-100">
          Appearance
        </p>
        <p class="text-sm text-surface-400">
          Toggle between light and dark theme.
        </p>
      </div>
      <button
        type="button"
        class="rounded-md border border-surface-200 px-3 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="toggle"
      >
        {{ isDark ? "Dark" : "Light" }}
      </button>
    </div>
  </div>
</template>
