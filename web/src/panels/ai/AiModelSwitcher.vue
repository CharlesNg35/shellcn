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

const providerChoices = computed(() => {
  const out: { label: string; value: string }[] = [];
  if (props.global?.configured) {
    out.push({ label: "Shared AI", value: "" });
  }
  for (const p of props.providers) {
    out.push({ label: p.name, value: p.id });
  }
  return out;
});

const selected = computed(
  () =>
    props.providers.find((p) => p.id === props.providerId) ??
    (!props.global?.configured && props.providerId === ""
      ? props.providers[0]
      : undefined),
);

const usingGlobal = computed(
  () => props.providerId === "" && !!props.global?.configured,
);
const effectiveProviderId = computed(
  () => selected.value?.id ?? props.providerId,
);

const modelChoices = computed(() => {
  const models = selected.value?.models ?? [];
  return models.map((m) => ({ label: m, value: m }));
});

const showModelSelect = computed(
  () => !usingGlobal.value && modelChoices.value.length > 1,
);
const activeProviderLabel = computed(() => {
  if (usingGlobal.value) return props.global?.provider ?? "Shared AI";
  return selected.value?.name ?? providerChoices.value[0]?.label ?? "";
});
const activeModel = computed(() => {
  if (usingGlobal.value) return props.global?.model ?? "";
  return props.model || selected.value?.defaultModel || "";
});

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
      v-else-if="activeProviderLabel"
      :value="activeProviderLabel"
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
      @update:model-value="emit('select', effectiveProviderId, $event)"
    />
    <Tag v-else-if="activeModel" :value="activeModel" severity="secondary" />
  </div>
</template>
