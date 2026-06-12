<script setup lang="ts">
import Tag from "primevue/tag";
import type { AiGlobalStatus } from "@/api/ai";
import AppIcon from "@/components/AppIcon.vue";
import { providerKindLabel } from "./providerKinds";

defineProps<{ global: AiGlobalStatus | null }>();
</script>

<template>
  <div class="flex min-h-0 flex-col gap-4">
    <div>
      <h2 class="text-base font-semibold text-surface-900 dark:text-surface-0">
        Shared AI
      </h2>
      <p class="text-sm text-surface-500 dark:text-surface-400">
        Operator-managed provider from server configuration.
      </p>
    </div>

    <div
      class="rounded-md border border-surface-200 p-4 dark:border-surface-800"
    >
      <div class="flex items-start gap-3">
        <div
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-300"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'bot' }" :size="18" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <p class="font-medium text-surface-900 dark:text-surface-0">
              {{ global?.provider || "Shared AI" }}
            </p>
            <Tag
              :value="global?.configured ? 'Configured' : 'Not configured'"
              :severity="global?.configured ? 'success' : 'secondary'"
            />
          </div>
          <p class="mt-1 text-sm text-surface-500 dark:text-surface-400">
            <template v-if="global?.configured">
              {{ providerKindLabel(global.kind || "") }} · {{ global.model }}
            </template>
            <template v-else>
              A shared workspace provider is not available.
            </template>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
