<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import { versionApi, type VersionInfo } from "../api/version";
import AppIcon from "./AppIcon.vue";

const version = ref<VersionInfo | null>(null);
const loading = ref(true);

const canCheck = computed(
  () => !!version.value && !version.value.dev && !version.value.checkDisabled,
);

async function load(refresh = false) {
  loading.value = true;
  try {
    version.value = await versionApi.get(refresh);
  } catch {
    version.value = null;
  } finally {
    loading.value = false;
  }
}

onMounted(() => load());
</script>

<template>
  <div
    class="flex items-center gap-3 rounded-lg border px-4 py-3 transition-colors"
    :class="
      version?.updateAvailable
        ? 'border-amber-300 bg-amber-50/60 dark:border-amber-500/40 dark:bg-amber-950/25'
        : 'border-surface-200 dark:border-surface-800'
    "
  >
    <AppIcon
      :icon="{ type: 'lucide', value: 'package' }"
      :size="18"
      class="text-surface-400"
    />
    <p
      class="min-w-0 flex-1 font-medium text-surface-800 dark:text-surface-100"
    >
      ShellCN
      <span
        class="ml-1 font-mono text-sm text-surface-500 dark:text-surface-400"
        >{{ version?.current ?? "—" }}</span
      >
    </p>

    <a
      v-if="version?.updateAvailable && version.releaseUrl"
      :href="version.releaseUrl"
      target="_blank"
      rel="noopener noreferrer"
      class="shrink-0"
    >
      <Button type="button" size="small" severity="warn">
        <AppIcon
          :icon="{ type: 'lucide', value: 'external-link' }"
          :size="14"
        />
        {{ version.latest }} available
      </Button>
    </a>
    <span
      v-else-if="version?.dev || version?.checkDisabled"
      class="shrink-0 rounded-full bg-surface-100 px-2.5 py-1 text-xs font-medium text-surface-500 dark:bg-surface-800 dark:text-surface-400"
    >
      {{ version?.dev ? "Development build" : "Updates disabled" }}
    </span>
    <span
      v-else-if="canCheck && !version?.error"
      class="shrink-0 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300"
    >
      Up to date
    </span>

    <Button
      v-if="canCheck"
      v-tooltip.left="'Check for updates'"
      type="button"
      severity="secondary"
      text
      rounded
      :disabled="loading"
      aria-label="Check for updates"
      @click="load(true)"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'refresh-cw' }"
        :size="15"
        :class="loading ? 'animate-spin' : ''"
      />
    </Button>
  </div>
</template>
