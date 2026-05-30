<script setup lang="ts">
import { onMounted, ref } from "vue";
import Button from "primevue/button";
import { useTheme } from "../composables/useTheme";
import { useAuthStore } from "../stores/auth";
import { adminSettingsApi } from "../api/admin";
import RoleGate from "../components/RoleGate.vue";
import AppIcon from "../components/AppIcon.vue";

const { isDark, toggle } = useTheme();
const auth = useAuthStore();

const emailEnabled = ref<boolean | null>(null);

onMounted(async () => {
  if (!auth.isAdmin) return;
  try {
    emailEnabled.value = (await adminSettingsApi.emailStatus()).enabled;
  } catch {
    emailEnabled.value = null;
  }
});

const linkClass =
  "flex items-center gap-3 rounded-lg border border-surface-200 px-4 py-3 transition-colors hover:bg-surface-100 focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none dark:border-surface-800 dark:hover:bg-surface-800/60";
</script>

<template>
  <div class="mx-auto flex max-w-2xl flex-col gap-4 p-8">
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      Settings
    </h1>

    <div
      class="flex items-center justify-between rounded-lg border border-surface-200 px-4 py-3 dark:border-surface-800"
    >
      <p class="font-medium text-surface-800 dark:text-surface-100">
        Appearance
      </p>
      <Button type="button" severity="secondary" outlined @click="toggle">
        {{ isDark ? "Dark" : "Light" }}
      </Button>
    </div>

    <RouterLink :to="{ name: 'activity' }" :class="linkClass">
      <AppIcon
        :icon="{ type: 'lucide', value: 'scroll-text' }"
        :size="18"
        class="text-surface-400"
      />
      <span
        class="min-w-0 flex-1 font-medium text-surface-800 dark:text-surface-100"
      >
        My activity
      </span>
      <AppIcon
        :icon="{ type: 'lucide', value: 'chevron-right' }"
        :size="16"
        class="text-surface-300"
      />
    </RouterLink>

    <RoleGate admin>
      <RouterLink :to="{ name: 'users' }" :class="linkClass">
        <AppIcon
          :icon="{ type: 'lucide', value: 'users' }"
          :size="18"
          class="text-surface-400"
        />
        <span
          class="min-w-0 flex-1 font-medium text-surface-800 dark:text-surface-100"
        >
          Users &amp; access
        </span>
        <AppIcon
          :icon="{ type: 'lucide', value: 'chevron-right' }"
          :size="16"
          class="text-surface-300"
        />
      </RouterLink>

      <div
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
    </RoleGate>
  </div>
</template>
