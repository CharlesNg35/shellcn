<script setup lang="ts">
import Button from "primevue/button";
import Tag from "primevue/tag";
import type { AiProviderSummary } from "../../api/ai";
import AppIcon from "../../components/AppIcon.vue";
import { btnGhost, btnPrimary } from "../../primevue/preset";
import { providerKindLabel } from "./providerKinds";

defineProps<{
  providers: AiProviderSummary[];
  loading: boolean;
}>();

const emit = defineEmits<{
  add: [];
  edit: [provider: AiProviderSummary];
  remove: [provider: AiProviderSummary];
}>();
</script>

<template>
  <div class="flex min-h-0 flex-col gap-4">
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0">
        <h2
          class="text-base font-semibold text-surface-900 dark:text-surface-0"
        >
          Provider configuration
        </h2>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          Manage personal providers and model allow-lists.
        </p>
      </div>
      <Button :pt="{ root: btnPrimary }" @click="emit('add')">
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="16" />
        Add provider
      </Button>
    </div>

    <div v-if="loading" class="grid gap-3" aria-busy="true">
      <div
        v-for="i in 3"
        :key="i"
        class="h-20 animate-pulse rounded-md border border-surface-200 bg-surface-100/60 dark:border-surface-800 dark:bg-surface-800/40"
      />
    </div>

    <div
      v-else-if="providers.length === 0"
      class="flex flex-col items-center gap-3 rounded-md border border-dashed border-surface-300 px-4 py-10 text-center dark:border-surface-700"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'sparkles' }"
        :size="28"
        class="text-surface-300"
      />
      <p class="text-sm text-surface-500 dark:text-surface-400">
        No personal providers yet. Add one to use the assistant with your own
        key.
      </p>
      <Button :pt="{ root: btnGhost }" @click="emit('add')">
        Add provider
      </Button>
    </div>

    <ul v-else class="grid gap-3">
      <li
        v-for="p in providers"
        :key="p.id"
        class="flex items-center gap-3 rounded-md border border-surface-200 px-4 py-3 dark:border-surface-800"
      >
        <div
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-300"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="17" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 items-center gap-2">
            <p
              class="truncate font-medium text-surface-900 dark:text-surface-0"
            >
              {{ p.name }}
            </p>
            <Tag
              :value="providerKindLabel(p.kind)"
              severity="secondary"
              class="shrink-0"
            />
          </div>
          <p class="truncate text-xs text-surface-500 dark:text-surface-400">
            {{ p.defaultModel }}{{ p.hasKey ? "" : " · no key" }}
          </p>
        </div>
        <Button
          text
          rounded
          severity="secondary"
          aria-label="Edit provider"
          @click="emit('edit', p)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="16" />
        </Button>
        <Button
          text
          rounded
          severity="danger"
          aria-label="Delete provider"
          @click="emit('remove', p)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="16" />
        </Button>
      </li>
    </ul>
  </div>
</template>
