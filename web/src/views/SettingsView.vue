<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useTheme } from "../composables/useTheme";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";

const { isDark, toggle } = useTheme();
const auth = useAuthStore();

const emailEnabled = ref<boolean | null>(null);

onMounted(async () => {
  if (!auth.isAdmin) return;
  try {
    const status = await api.get<{ enabled: boolean }>("/admin/email");
    emailEnabled.value = status.enabled;
  } catch {
    emailEnabled.value = null;
  }
});
</script>

<template>
  <div class="mx-auto flex max-w-2xl flex-col gap-4 p-8">
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
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

    <div
      v-if="auth.isAdmin"
      class="flex items-center justify-between rounded-lg border border-surface-200 px-4 py-3 dark:border-surface-800"
    >
      <p class="font-medium text-surface-800 dark:text-surface-100">Email</p>
      <span
        v-if="emailEnabled !== null"
        class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium"
        :class="
          emailEnabled
            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
            : 'bg-surface-100 text-surface-500 dark:bg-surface-800'
        "
      >
        {{ emailEnabled ? "Configured" : "Not configured" }}
      </span>
    </div>
  </div>
</template>
