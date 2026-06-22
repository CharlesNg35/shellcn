<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Select from "primevue/select";
import Button from "primevue/button";
import { credentialsApi } from "@/api/credentials";
import CredentialFormDialog from "@/components/CredentialFormDialog.vue";
import AppIcon from "@/components/AppIcon.vue";
import type {
  CredentialKindInfo,
  CredentialRefState,
  CredentialSelector,
  CredentialSummary,
} from "@/types/projection";

const props = defineProps<{
  selector: CredentialSelector;
  protocol?: string;
  modelValue?: string;
  state?: CredentialRefState;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const options = ref<CredentialSummary[]>([]);
const kindCatalog = ref<CredentialKindInfo[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const showCreate = ref(false);
const replacingHidden = ref(false);
const requestProtocol = computed(
  () => props.protocol ?? props.selector.protocols?.[0] ?? "",
);

const choices = computed(() =>
  options.value.map((c) => ({
    value: c.id,
    label: `${c.name} · ${kindLabel(c.kind)}${summaryLabel(c)}`,
  })),
);

function kindLabel(kind: string): string {
  return kindCatalog.value.find((k) => k.kind === kind)?.label ?? kind;
}

function summaryLabel(c: CredentialSummary): string {
  const kind = kindCatalog.value.find((k) => k.kind === c.kind);
  const values = c.values ?? {};
  const summary = (kind?.fields ?? [])
    .filter((field) => field.public && values[field.key])
    .map((field) => values[field.key])
    .join(", ");
  return summary ? ` (${summary})` : "";
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    const [nextOptions, nextKinds] = await Promise.all([
      credentialsApi.list({
        kind: props.selector.kind || undefined,
        protocol: requestProtocol.value || undefined,
      }),
      kindCatalog.value.length
        ? Promise.resolve(kindCatalog.value)
        : credentialsApi.kinds(),
    ]);
    options.value = Array.isArray(nextOptions) ? nextOptions : [];
    kindCatalog.value = Array.isArray(nextKinds) ? nextKinds : [];
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function onCreated(credential?: CredentialSummary): Promise<void> {
  await load();
  if (credential?.id) {
    replacingHidden.value = true;
    emit("update:modelValue", credential.id);
  }
}

function replaceHidden(): void {
  replacingHidden.value = true;
  emit("update:modelValue", "");
}

watch(() => [props.selector, requestProtocol.value], load, { immediate: true });
</script>

<template>
  <div>
    <div
      v-if="state?.state === 'set' && !state.readable && !replacingHidden"
      class="flex items-center justify-between gap-3 rounded-md border border-surface-300 px-3 py-2 text-sm dark:border-surface-700"
    >
      <span
        class="flex min-w-0 items-center gap-2 text-surface-600 dark:text-surface-300"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'lock' }" :size="14" />
        <span class="truncate">Credential configured</span>
      </span>
      <Button link class="shrink-0 text-xs!" @click="replaceHidden">
        Replace
      </Button>
    </div>

    <Select
      v-else
      :model-value="modelValue"
      :options="choices"
      option-label="label"
      option-value="value"
      :loading="loading"
      :placeholder="loading ? 'Loading credentials…' : 'Select a credential'"
      @update:model-value="emit('update:modelValue', $event)"
    />
    <div class="mt-1 flex items-center justify-between gap-2">
      <span
        v-if="error"
        class="flex min-w-0 items-center gap-2 text-xs text-red-500"
      >
        <span class="truncate" role="alert">{{ error }}</span>
        <Button link class="shrink-0 text-xs!" @click="load">Retry</Button>
      </span>
      <p
        v-else-if="!loading && !options.length"
        class="text-xs text-surface-400"
      >
        No matching credentials yet.
      </p>
      <span v-else />
      <Button link class="shrink-0 text-xs!" @click="showCreate = true">
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="12" />
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
