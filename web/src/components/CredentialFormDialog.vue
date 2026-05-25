<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import MultiSelect from "primevue/multiselect";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Textarea from "primevue/textarea";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useConnectionsStore } from "../stores/connections";
import { useNotify } from "../composables/useNotify";
import { dialogRoot, btnPrimary, btnGhost } from "../primevue/preset";
import type {
  CredentialKindInfo,
  CredentialSelector,
  CredentialSummary,
} from "../types/projection";

const props = defineProps<{
  visible: boolean;
  credential?: CredentialSummary | null;
  selector?: CredentialSelector;
  protocol?: string;
}>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [credential?: CredentialSummary];
}>();

const conns = useConnectionsStore();
const notify = useNotify();

const isEdit = computed(() => Boolean(props.credential));
const name = ref("");
const kind = ref("");
const identity = ref("");
const protocols = ref<string[]>([]);
const secret = ref("");
const replacing = ref(true);
const errors = ref<Record<string, string>>({});
const busy = ref(false);
const kindCatalog = ref<CredentialKindInfo[]>([]);
const catalogLoading = ref(false);
const catalogError = ref<string | null>(null);

const selectorKinds = computed(() => props.selector?.kinds ?? []);
const scopedToSelector = computed(
  () =>
    !isEdit.value &&
    (selectorKinds.value.length > 0 || Boolean(props.protocol)),
);
const kindOptions = computed(() => {
  const allowed = new Set(selectorKinds.value);
  return kindCatalog.value
    .filter((k) => !allowed.size || allowed.has(k.kind))
    .map((k) => ({ label: k.label, value: k.kind }));
});
const showKindSelect = computed(
  () => !scopedToSelector.value || kindOptions.value.length > 1,
);
const kindDisplayLabel = computed(
  () =>
    kindOptions.value.find((k) => k.value === kind.value)?.label ?? kind.value,
);
const selectedKind = computed(
  () => kindCatalog.value.find((k) => k.kind === kind.value) ?? null,
);
const compatibleProtocols = computed(
  () => selectedKind.value?.compatibleProtocols ?? [],
);
const protocolOptions = computed(() => {
  const selectorProtocols = props.selector?.protocols ?? [];
  const allowed = conns.plugins.filter((p) => {
    if (
      compatibleProtocols.value.length &&
      !compatibleProtocols.value.includes(p.name)
    ) {
      return false;
    }
    return !selectorProtocols.length || selectorProtocols.includes(p.name);
  });
  return allowed.map((p) => ({ label: p.title, value: p.name }));
});
const multiline = computed(() => selectedKind.value?.secretMultiline === true);
const secretLabel = computed(
  () => selectedKind.value?.secretLabel ?? "Secret material",
);
const identityLabel = computed(() => selectedKind.value?.identityLabel ?? "");
const showIdentity = computed(() => identityLabel.value !== "");

async function loadKindCatalog(): Promise<void> {
  if (kindCatalog.value.length || catalogLoading.value) return;
  catalogLoading.value = true;
  catalogError.value = null;
  try {
    const catalog = await api.get<CredentialKindInfo[]>("/credential-kinds");
    kindCatalog.value = Array.isArray(catalog) ? catalog : [];
  } catch (e) {
    catalogError.value = (e as Error).message;
  } finally {
    catalogLoading.value = false;
  }
}

function firstAllowedKind(): string {
  const current = kindOptions.value.find(
    (option) => option.value === kind.value,
  );
  if (current) return current.value;
  if (props.credential?.kind) return props.credential.kind;
  return kindOptions.value[0]?.value ?? "";
}

function defaultProtocols(): string[] {
  const options = new Set(protocolOptions.value.map((p) => p.value));
  if (props.protocol && options.has(props.protocol)) return [props.protocol];
  if (props.selector?.protocols?.length === 1) {
    const only = props.selector.protocols[0];
    if (options.has(only)) return [only];
  }
  return [];
}

function normalizeForKind(): void {
  if (
    kindOptions.value.length &&
    !kindOptions.value.some((k) => k.value === kind.value)
  ) {
    kind.value = firstAllowedKind();
  }
  if (!showIdentity.value) identity.value = "";
  const allowedProtocols = new Set(protocolOptions.value.map((p) => p.value));
  protocols.value = protocols.value.filter((p) => allowedProtocols.has(p));
  if (!isEdit.value && protocols.value.length === 0) {
    protocols.value = defaultProtocols();
  }
}

watch(
  () => props.visible,
  async (open) => {
    if (!open) return;
    await loadKindCatalog();
    if (catalogError.value) return;
    errors.value = {};
    secret.value = "";
    if (props.credential) {
      name.value = props.credential.name;
      kind.value = props.credential.kind;
      identity.value =
        props.credential.identity ??
        (props.credential as CredentialSummary & { username?: string })
          .username ??
        "";
      protocols.value = props.credential.protocols ?? [];
      replacing.value = false;
    } else {
      name.value = "";
      kind.value = firstAllowedKind();
      identity.value = "";
      protocols.value = defaultProtocols();
      replacing.value = true;
    }
    normalizeForKind();
  },
  { immediate: true },
);

watch(kind, normalizeForKind);

function validate(): boolean {
  const next: Record<string, string> = {};
  if (catalogLoading.value || catalogError.value) return false;
  if (!name.value.trim()) next.name = "A name is required.";
  if (!kind.value) next.kind = "A kind is required.";
  if (!isEdit.value && !secret.value.trim())
    next.secret = "Secret material is required.";
  errors.value = next;
  return Object.keys(next).length === 0;
}

async function save(): Promise<void> {
  if (!validate()) return;
  busy.value = true;
  const body = {
    name: name.value.trim(),
    kind: kind.value,
    identity: showIdentity.value ? identity.value.trim() : undefined,
    protocols: protocols.value.length ? protocols.value : undefined,
    // Blank secret on edit keeps the stored material (write-only).
    secret: replacing.value ? secret.value : "",
  };
  try {
    if (isEdit.value && props.credential) {
      const updated = await api.put<CredentialSummary>(
        `/credentials/${props.credential.id}`,
        body,
      );
      notify.success("Credential updated", name.value);
      emit("saved", updated);
    } else {
      const created = await api.post<CredentialSummary>("/credentials", body);
      notify.success("Credential created", name.value);
      emit("saved", created);
    }
    emit("update:visible", false);
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) {
      notify.error("Could not save credential", e.message);
    }
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="isEdit ? 'Edit credential' : 'New credential'"
    :closable="!busy"
    :pt="{
      root: dialogRoot(),
      content: 'max-h-[70vh] overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="flex flex-col gap-4">
      <div v-if="catalogLoading" class="flex flex-col gap-3">
        <div
          v-for="i in 5"
          :key="i"
          class="h-10 animate-pulse rounded-md bg-surface-100 dark:bg-surface-800"
        />
      </div>
      <p v-else-if="catalogError" class="text-sm text-red-500">
        {{ catalogError }}
      </p>
      <div v-else class="flex flex-col gap-1.5">
        <label
          for="cred-name"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Name <span class="text-red-500">*</span>
        </label>
        <InputText
          id="cred-name"
          :model-value="name"
          placeholder="e.g. ops shared key"
          @update:model-value="name = $event ?? ''"
        />
        <p v-if="errors.name" class="text-xs text-red-500">{{ errors.name }}</p>
      </div>

      <div
        v-if="!catalogLoading && !catalogError"
        class="flex flex-col gap-1.5"
      >
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Kind <span class="text-red-500">*</span>
        </label>
        <Select
          v-if="showKindSelect"
          :model-value="kind"
          :options="kindOptions"
          option-label="label"
          option-value="value"
          @update:model-value="kind = $event"
        />
        <div
          v-else
          class="rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-sm text-surface-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-200"
        >
          {{ kindDisplayLabel }}
        </div>
      </div>

      <div
        v-if="!catalogLoading && !catalogError && showIdentity"
        class="flex flex-col gap-1.5"
      >
        <label
          for="cred-identity"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          {{ identityLabel }}
        </label>
        <InputText
          id="cred-identity"
          :model-value="identity"
          :placeholder="identityLabel"
          @update:model-value="identity = $event ?? ''"
        />
      </div>

      <div
        v-if="!catalogLoading && !catalogError && !scopedToSelector"
        class="flex flex-col gap-1.5"
      >
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Allowed protocols
        </label>
        <MultiSelect
          :model-value="protocols"
          :options="protocolOptions"
          option-label="label"
          option-value="value"
          display="chip"
          :placeholder="'Any compatible protocol'"
          @update:model-value="protocols = $event"
        />
      </div>

      <div
        v-if="!catalogLoading && !catalogError"
        class="flex flex-col gap-1.5"
      >
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          {{ secretLabel }}
          <span v-if="!isEdit" class="text-red-500">*</span>
        </label>

        <Button
          v-if="isEdit && !replacing"
          type="button"
          :pt="{
            root: 'flex w-full items-center justify-between rounded-md border border-surface-300 px-2.5 py-1.5 text-sm text-surface-500 dark:border-surface-700',
          }"
          @click="replacing = true"
        >
          <span>•••••••• Set</span>
          <span class="text-xs text-primary-500">Replace</span>
        </Button>
        <Textarea
          v-else-if="multiline"
          :model-value="secret"
          rows="5"
          class="font-mono"
          :placeholder="`Paste ${secretLabel.toLowerCase()}`"
          @update:model-value="secret = $event ?? ''"
        />
        <Password
          v-else
          :model-value="secret"
          :feedback="false"
          toggle-mask
          :input-props="{ autocomplete: 'new-password' }"
          @update:model-value="secret = $event ?? ''"
        />
        <p v-if="errors.secret" class="text-xs text-red-500">
          {{ errors.secret }}
        </p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{ root: btnGhost }"
          @click="emit('update:visible', false)"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :disabled="busy || catalogLoading || Boolean(catalogError)"
          :pt="{ root: btnPrimary }"
          @click="save"
        >
          {{ busy ? "Saving…" : isEdit ? "Save changes" : "Create credential" }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>
