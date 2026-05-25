<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Select from "primevue/select";
import { api } from "../../api/client";
import type {
  CredentialSelector,
  CredentialSummary,
} from "../../types/projection";

const props = defineProps<{
  selector: CredentialSelector;
  protocol?: string;
  modelValue?: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const options = ref<CredentialSummary[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

const choices = computed(() =>
  options.value.map((c) => ({
    value: c.id,
    label: `${c.name} · ${c.kind}${c.username ? ` (${c.username})` : ""}`,
  })),
);

onMounted(async () => {
  const sp = new URLSearchParams();
  if (props.selector.kinds.length)
    sp.set("kind", props.selector.kinds.join(","));
  const protocol = props.protocol ?? props.selector.protocols?.[0];
  if (protocol) sp.set("protocol", protocol);
  try {
    options.value = await api.get<CredentialSummary[]>(
      `/credentials?${sp.toString()}`,
    );
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div>
    <Select
      :model-value="modelValue"
      :options="choices"
      option-label="label"
      option-value="value"
      :loading="loading"
      :placeholder="loading ? 'Loading credentials…' : 'Select a credential'"
      @update:model-value="emit('update:modelValue', $event)"
    />
    <p v-if="error" class="mt-1 text-xs text-red-500">{{ error }}</p>
    <p
      v-else-if="!loading && !options.length"
      class="mt-1 text-xs text-surface-400"
    >
      No matching credentials. Create one first.
    </p>
  </div>
</template>
