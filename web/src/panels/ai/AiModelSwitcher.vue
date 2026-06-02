<script setup lang="ts">
import { computed } from "vue";
import Tag from "primevue/tag";
import Select from "primevue/select";
import type { AiGlobalStatus, AiProviderSummary } from "../../api/ai";

const props = defineProps<{
  providers: AiProviderSummary[];
  global: AiGlobalStatus | null;
  providerId: string;
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
const activeProviderLabel = computed(() => {
  if (usingGlobal.value) return props.global?.provider ?? "Shared AI";
  return selected.value?.name ?? providerChoices.value[0]?.label ?? "";
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
  </div>
</template>
