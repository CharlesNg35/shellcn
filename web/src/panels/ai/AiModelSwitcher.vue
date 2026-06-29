<script setup lang="ts">
import { computed } from "vue";
import Select from "primevue/select";
import Tag from "primevue/tag";
import AppIcon from "@/components/AppIcon.vue";
import type { AiGlobalStatus, AiProviderSummary } from "@/api/ai";

const props = defineProps<{
  providers: AiProviderSummary[];
  global: AiGlobalStatus | null;
  providerId: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ select: [providerId: string] }>();

type ProviderChoice = { label: string; value: string; model: string };

// PrimeVue Select treats an empty-string value as "no selection" and renders a
// blank label, so the shared/global provider (providerId "") needs a sentinel.
const SHARED = "__shared__";
const toValue = (id: string): string => (id === "" ? SHARED : id);
const fromValue = (value: string): string => (value === SHARED ? "" : value);

const providerChoices = computed(() => {
  const out: ProviderChoice[] = [];
  if (props.global?.configured) {
    out.push({
      label: props.global.provider ?? "Shared AI",
      value: SHARED,
      model: props.global.model ?? "",
    });
  }
  for (const p of props.providers) {
    out.push({ label: p.name, value: p.id, model: p.model });
  }
  return out;
});

const activeChoice = computed(
  () =>
    providerChoices.value.find(
      (choice) => choice.value === toValue(props.providerId),
    ) ?? providerChoices.value[0],
);
const selectedProviderId = computed(() => activeChoice.value?.value ?? SHARED);

const canSwitch = computed(() => providerChoices.value.length > 1);
const activeTitle = computed(() => choiceTitle(activeChoice.value));

function choiceTitle(choice?: ProviderChoice): string | undefined {
  if (!choice) return undefined;
  return choice.model ? `${choice.label} - ${choice.model}` : choice.label;
}

function optionPt({ context }: { context: { option?: ProviderChoice } }): {
  title?: string;
} {
  return { title: choiceTitle(context.option) };
}

function pickProvider(value: string): void {
  emit("select", fromValue(value));
}
</script>

<template>
  <div v-if="activeChoice" class="min-w-0">
    <Select
      v-if="canSwitch"
      :model-value="selectedProviderId"
      :options="providerChoices"
      option-label="label"
      option-value="value"
      append-to="body"
      scroll-height="12rem"
      :disabled="disabled"
      aria-label="AI provider"
      :title="activeTitle"
      class="max-w-full"
      :pt="{
        root: 'flex h-8 max-w-full cursor-default w-full min-w-0 items-center overflow-hidden rounded-md border border-surface-300 bg-surface-0 text-xs shadow-sm transition-colors focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-950',
        label:
          'flex h-full min-w-0 flex-1 items-center truncate px-2.5 text-left text-xs leading-none text-surface-700 dark:text-surface-100',
        dropdown:
          'flex h-full w-7 shrink-0 items-center justify-center text-surface-400',
        option: optionPt,
      }"
      @update:model-value="pickProvider"
    >
      <template #dropdownicon>
        <AppIcon :icon="{ type: 'lucide', value: 'chevron-down' }" :size="13" />
      </template>
    </Select>
    <Tag
      v-else
      severity="secondary"
      :value="activeChoice.label"
      :title="activeTitle"
      class="max-w-full"
      :pt="{
        root: 'flex h-8 max-w-full items-center px-2',
        label: 'truncate text-xs leading-none',
      }"
    />
  </div>
</template>
