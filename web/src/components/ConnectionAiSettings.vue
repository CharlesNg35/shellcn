<script setup lang="ts">
import Select from "primevue/select";
import Checkbox from "primevue/checkbox";
import AppIcon from "./AppIcon.vue";

defineProps<{
  configured: boolean;
  mode: string;
  allowDestructive: boolean;
  autoApprove: boolean;
}>();

const emit = defineEmits<{
  "update:mode": [value: string];
  "update:allowDestructive": [value: boolean];
  "update:autoApprove": [value: boolean];
}>();

const aiModeChoices = [
  { label: "Disabled", value: "disabled" },
  { label: "Read-only", value: "read_only" },
  { label: "Read & write", value: "read_write" },
];
</script>

<template>
  <fieldset
    class="flex min-w-0 flex-col gap-3 rounded-md border border-surface-200 p-3 dark:border-surface-700"
  >
    <legend
      class="flex items-center gap-1.5 px-1 text-sm font-medium text-surface-700 dark:text-surface-200"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="14" />
      AI assistant
    </legend>
    <p
      v-if="!configured"
      class="text-xs text-surface-500 dark:text-surface-400"
    >
      Configure an AI provider in
      <RouterLink
        :to="{ name: 'ai-settings' }"
        class="text-primary-500 underline"
        >Settings → AI providers</RouterLink
      >
      to enable the assistant for connections.
    </p>
    <template v-else>
      <div class="flex items-center justify-between gap-3">
        <span class="text-sm text-surface-700 dark:text-surface-200"
          >Assistant access</span
        >
        <div class="w-44 shrink-0">
          <Select
            :model-value="mode || 'read_only'"
            :options="aiModeChoices"
            option-label="label"
            option-value="value"
            aria-label="AI assistant access"
            @update:model-value="emit('update:mode', $event)"
          />
        </div>
      </div>
      <label
        v-if="mode === 'read_write'"
        class="flex items-start gap-2 text-sm"
      >
        <Checkbox
          :model-value="allowDestructive"
          binary
          input-id="ai-allow-destructive"
          @update:model-value="emit('update:allowDestructive', $event)"
        />
        <span class="flex min-w-0 flex-col">
          <span class="text-surface-700 dark:text-surface-200"
            >Allow destructive operations</span
          >
          <span class="text-xs text-amber-600 dark:text-amber-400">
            Lets the assistant delete/drop/truncate. Confirmation depends on the
            approval policy below.
          </span>
        </span>
      </label>
      <label
        v-if="mode === 'read_write'"
        class="flex items-start gap-2 text-sm"
      >
        <Checkbox
          :model-value="autoApprove"
          binary
          input-id="ai-auto-approve"
          @update:model-value="emit('update:autoApprove', $event)"
        />
        <span class="flex min-w-0 flex-col">
          <span class="text-surface-700 dark:text-surface-200"
            >Auto-approve assistant actions</span
          >
          <span class="text-xs text-surface-500 dark:text-surface-400">
            Run allowed write and destructive actions without asking for
            approval each time.
          </span>
        </span>
      </label>
    </template>
  </fieldset>
</template>
