<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Select from "primevue/select";
import { api } from "../../api/client";
import CredentialFormDialog from "../../components/CredentialFormDialog.vue";
import AppIcon from "../../components/AppIcon.vue";
import type {
  CredentialKind,
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
const createKind = computed<CredentialKind | undefined>(
  () => props.selector.kinds[0],
);
const createSelector = computed<CredentialSelector>(() => ({
  ...props.selector,
  kinds: createKind.value ? [createKind.value] : props.selector.kinds,
}));

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
}

async function onCreated(credential?: CredentialSummary): Promise<void> {
  await load();
  if (credential?.id) {
    emit("update:modelValue", credential.id);
  }
}

onMounted(load);
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
      <button
        type="button"
        class="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
        @click="showCreate = true"
      >
        <AppIcon :icon="{ type: 'name', value: 'plus' }" :size="12" />
        New credential
      </button>
    </div>

    <!-- Create a credential without leaving the connection form; on save the
         list reloads so the new one is immediately selectable. Lazily mounted so
         this heavy dialog (and its store) only loads when actually opened. -->
    <CredentialFormDialog
      v-if="showCreate"
      v-model:visible="showCreate"
      :selector="createSelector"
      :protocol="protocol"
      :locked-kind="createKind"
      @saved="onCreated"
    />
  </div>
</template>
