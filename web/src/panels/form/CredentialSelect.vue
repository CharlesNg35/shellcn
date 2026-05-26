<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Select from "primevue/select";
import Button from "primevue/button";
import { api } from "../../api/client";
import CredentialFormDialog from "../../components/CredentialFormDialog.vue";
import AppIcon from "../../components/AppIcon.vue";
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
const showCreate = ref(false);
const requestProtocol = computed(
  () => props.protocol ?? props.selector.protocols?.[0] ?? "",
);

const choices = computed(() =>
  options.value.map((c) => ({
    value: c.id,
    label: `${c.name} · ${c.kind}${c.identity ? ` (${c.identity})` : ""}`,
  })),
);

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  const sp = new URLSearchParams();
  if (props.selector.kinds.length)
    sp.set("kind", props.selector.kinds.join(","));
  if (requestProtocol.value) sp.set("protocol", requestProtocol.value);
  try {
    options.value = await api.get<CredentialSummary[]>(
      `/credentials${sp.toString() ? `?${sp.toString()}` : ""}`,
    );
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function onCreated(credential?: CredentialSummary): Promise<void> {
  await load();
  if (credential?.id) {
    emit("update:modelValue", credential.id);
  }
}

watch(() => [props.selector, requestProtocol.value], load, { immediate: true });
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
    <div class="mt-1 flex items-center justify-between gap-2">
      <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
      <p
        v-else-if="!loading && !options.length"
        class="text-xs text-surface-400"
      >
        No matching credentials yet.
      </p>
      <span v-else />
      <Button link class="shrink-0 text-xs!" @click="showCreate = true">
        <AppIcon :icon="{ type: 'name', value: 'plus' }" :size="12" />
        New credential
      </Button>
    </div>

    <!-- Create a credential without leaving the connection form; on save the
         list reloads so the new one is immediately selectable. Lazily mounted so
         this heavy dialog (and its store) only loads when actually opened. -->
    <CredentialFormDialog
      v-if="showCreate"
      v-model:visible="showCreate"
      :selector="selector"
      :protocol="requestProtocol"
      @saved="onCreated"
    />
  </div>
</template>
