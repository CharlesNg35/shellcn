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
import type { CredentialSummary } from "../types/projection";

const props = defineProps<{
  visible: boolean;
  credential?: CredentialSummary | null;
}>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [];
}>();

const conns = useConnectionsStore();
const notify = useNotify();

const kindOptions = [
  { label: "SSH private key", value: "ssh_private_key" },
  { label: "SSH password", value: "ssh_password" },
  { label: "TLS client certificate", value: "tls_client_cert" },
  { label: "Database password", value: "db_password" },
  { label: "API token", value: "api_token" },
];
const multilineKinds = new Set(["ssh_private_key", "tls_client_cert"]);

const isEdit = computed(() => Boolean(props.credential));
const name = ref("");
const kind = ref("ssh_password");
const username = ref("");
const protocols = ref<string[]>([]);
const secret = ref("");
const replacing = ref(true);
const errors = ref<Record<string, string>>({});
const busy = ref(false);

const protocolOptions = computed(() =>
  conns.plugins.map((p) => ({ label: p.title, value: p.name })),
);
const multiline = computed(() => multilineKinds.has(kind.value));

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    errors.value = {};
    secret.value = "";
    if (props.credential) {
      name.value = props.credential.name;
      kind.value = props.credential.kind;
      username.value = props.credential.username ?? "";
      protocols.value = props.credential.protocols ?? [];
      replacing.value = false;
    } else {
      name.value = "";
      kind.value = "ssh_password";
      username.value = "";
      protocols.value = [];
      replacing.value = true;
    }
  },
  { immediate: true },
);

function validate(): boolean {
  const next: Record<string, string> = {};
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
    username: username.value.trim() || undefined,
    protocols: protocols.value.length ? protocols.value : undefined,
    // Blank secret on edit keeps the stored material (write-only).
    secret: replacing.value ? secret.value : "",
  };
  try {
    if (isEdit.value && props.credential) {
      await api.put(`/credentials/${props.credential.id}`, body);
      notify.success("Credential updated", name.value);
    } else {
      await api.post("/credentials", body);
      notify.success("Credential created", name.value);
    }
    emit("saved");
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
      <div class="flex flex-col gap-1.5">
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

      <div class="flex flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Kind <span class="text-red-500">*</span>
        </label>
        <Select
          :model-value="kind"
          :options="kindOptions"
          option-label="label"
          option-value="value"
          @update:model-value="kind = $event"
        />
      </div>

      <div class="flex flex-col gap-1.5">
        <label
          for="cred-username"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Username
        </label>
        <InputText
          id="cred-username"
          :model-value="username"
          placeholder="optional"
          @update:model-value="username = $event ?? ''"
        />
      </div>

      <div class="flex flex-col gap-1.5">
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

      <div class="flex flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Secret material
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
          placeholder="Paste the key or certificate"
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
          :disabled="busy"
          :pt="{ root: btnPrimary }"
          @click="save"
        >
          {{ busy ? "Saving…" : isEdit ? "Save changes" : "Create credential" }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>
