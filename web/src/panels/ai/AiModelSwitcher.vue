<script setup lang="ts">
import { computed } from "vue";
import Select from "primevue/select";
import Tag from "primevue/tag";
import type { AiGlobalStatus, AiProviderSummary } from "../../api/ai";

const props = defineProps<{
  providers: AiProviderSummary[];
  global: AiGlobalStatus | null;
  providerId: string;
  model: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ select: [providerId: string, model: string] }>();

// Provider choices: the shared/global config (locked model) plus each user provider.
const providerChoices = computed(() => {
  const out: { label: string; value: string }[] = [];
  if (props.global?.configured) {
    out.push({ label: `Shared · ${props.global.provider}`, value: "" });
  }
  for (const p of props.providers) {
    out.push({ label: p.name, value: p.id });
  }
  return out;
});

const selected = computed(() =>
  props.providers.find((p) => p.id === props.providerId),
);

const usingGlobal = computed(
  () => props.providerId === "" && !!props.global?.configured,
);

const modelChoices = computed(() => {
  const models = selected.value?.models ?? [];
  return models.map((m) => ({ label: m, value: m }));
});

// Only the user's own providers allow switching the model; the global model is pinned.
const showModelSelect = computed(
  () => !usingGlobal.value && modelChoices.value.length > 1,
);

function pickProvider(id: string): void {
  const p = props.providers.find((x) => x.id === id);
  emit("select", id, p?.defaultModel ?? "");
}
</script>

<template>
  <div class="flex items-center gap-1.5">
    <Select
      v-if="providerChoices.length > 1"
      :model-value="providerId"
      :options="providerChoices"
      option-label="label"
      option-value="value"
      :disabled="disabled"
      aria-label="AI provider"
      :pt="{ root: 'text-xs' }"
      @update:model-value="pickProvider"
    />
    <Tag
      v-else-if="usingGlobal"
      :value="`Shared · ${global?.model}`"
      severity="secondary"
    />

    <Select
      v-if="showModelSelect"
      :model-value="model || selected?.defaultModel"
      :options="modelChoices"
      option-label="label"
      option-value="value"
      :disabled="disabled"
      aria-label="Model"
      :pt="{ root: 'text-xs' }"
      @update:model-value="emit('select', providerId, $event)"
    />
    <span
      v-else-if="usingGlobal && global?.model"
      class="text-xs text-surface-400"
      :title="`Shared model: ${global.model}`"
    />
  </div>
</template>
