<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useConnectionsStore } from "../stores/connections";
import { useNotify } from "../composables/useNotify";
import SchemaForm from "../panels/form/SchemaForm.vue";
import AppIcon from "./AppIcon.vue";
import type {
  ConnectionDetail,
  ConnectionSummary,
  PluginProjection,
  Transport,
} from "../types/projection";

const props = defineProps<{ visible: boolean; connectionId?: string | null }>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [payload: { id: string; created: boolean }];
}>();

const conns = useConnectionsStore();
const notify = useNotify();

const isEdit = computed(() => Boolean(props.connectionId));
const protocol = ref("");
const projection = ref<PluginProjection | null>(null);
const name = ref("");
const nameError = ref<string | null>(null);
const transport = ref<Transport>("direct");
const configModel = ref<Record<string, unknown>>({});
const secretsSet = ref<Record<string, boolean>>({});
const loading = ref(false);
const busy = ref(false);
const formRef = ref<{ submit: () => void } | null>(null);

const pluginChoices = computed(() =>
  conns.plugins.map((p) => ({ label: p.title, value: p.name })),
);
const transportChoices = computed(() =>
  (projection.value?.supportedTransports ?? ["direct"]).map((t) => ({
    label: t === "agent" ? "Agent" : "Direct",
    value: t,
  })),
);

function reset(): void {
  protocol.value = "";
  projection.value = null;
  name.value = "";
  nameError.value = null;
  transport.value = "direct";
  configModel.value = {};
  secretsSet.value = {};
}

async function selectPlugin(nextProtocol: string): Promise<void> {
  protocol.value = nextProtocol;
  projection.value = await conns.projection(nextProtocol);
  transport.value = projection.value.supportedTransports[0] ?? "direct";
}

async function loadForEdit(id: string): Promise<void> {
  loading.value = true;
  try {
    const detail = await api.get<ConnectionDetail>(`/connections/${id}`);
    name.value = detail.name;
    transport.value = detail.transport;
    configModel.value = detail.config ?? {};
    secretsSet.value = Object.fromEntries(
      Object.entries(detail.secrets ?? {}).map(([k, v]) => [k, v === "set"]),
    );
    protocol.value = detail.protocol;
    projection.value = await conns.projection(detail.protocol);
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    reset();
    if (props.connectionId) void loadForEdit(props.connectionId);
  },
  { immediate: true },
);

function close(): void {
  emit("update:visible", false);
}

function requestSubmit(): void {
  nameError.value = name.value.trim() ? null : "A name is required.";
  if (nameError.value) return;
  formRef.value?.submit();
}

async function onConfig(config: Record<string, unknown>): Promise<void> {
  busy.value = true;
  try {
    if (isEdit.value && props.connectionId) {
      const updated = await api.put<ConnectionDetail>(
        `/connections/${props.connectionId}`,
        { name: name.value.trim(), transport: transport.value, config },
      );
      await conns.refresh();
      notify.success("Connection updated", updated.name);
      emit("saved", { id: props.connectionId, created: false });
    } else {
      const created = await api.post<ConnectionSummary>("/connections", {
        name: name.value.trim(),
        protocol: protocol.value,
        transport: transport.value,
        config,
      });
      await conns.refresh();
      notify.success("Connection created", created.name);
      emit("saved", { id: created.id, created: true });
    }
    close();
  } catch (e) {
    if (e instanceof ApiError && (e.status === 400 || e.status === 409)) {
      notify.error("Could not save connection", e.message);
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
    :header="isEdit ? 'Edit connection' : 'Add connection'"
    :closable="!busy"
    :pt="{
      root: 'w-full max-w-lg rounded-lg bg-surface-0 shadow-xl dark:bg-surface-900',
      content: 'max-h-[70vh] overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <p v-if="loading" class="py-8 text-center text-sm text-surface-400">
      Loading…
    </p>

    <div v-else class="flex flex-col gap-5">
      <!-- Protocol picker (create only; fixed on edit). -->
      <div v-if="!isEdit" class="flex flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Protocol <span class="text-red-500">*</span>
        </label>
        <Select
          :model-value="protocol"
          :options="pluginChoices"
          option-label="label"
          option-value="value"
          placeholder="Choose a protocol"
          @update:model-value="selectPlugin"
        />
      </div>

      <template v-if="projection">
        <div class="flex flex-col gap-1.5">
          <label
            for="conn-name"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Name <span class="text-red-500">*</span>
          </label>
          <InputText
            id="conn-name"
            :model-value="name"
            placeholder="e.g. prod-web-01"
            @update:model-value="name = $event ?? ''"
          />
          <p v-if="nameError" class="text-xs text-red-500">{{ nameError }}</p>
        </div>

        <div v-if="transportChoices.length > 1" class="flex flex-col gap-1.5">
          <label
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Transport
          </label>
          <Select
            :model-value="transport"
            :options="transportChoices"
            option-label="label"
            option-value="value"
            @update:model-value="transport = $event"
          />
        </div>

        <SchemaForm
          :key="`${protocol}:${isEdit ? 'edit' : 'create'}`"
          ref="formRef"
          :schema="projection.config"
          :model-value="configModel"
          :secrets-set="secretsSet"
          :protocol="protocol"
          @submit="onConfig"
        />
      </template>

      <p v-else-if="!isEdit" class="text-sm text-surface-400">
        Pick a protocol to configure the connection.
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{
            root: 'rounded-md px-3 py-1.5 text-sm text-surface-600 hover:bg-surface-100 dark:text-surface-300 dark:hover:bg-surface-800',
          }"
          @click="close"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :disabled="busy || !projection"
          :pt="{
            root: 'flex items-center gap-1.5 rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50',
          }"
          @click="requestSubmit"
        >
          <AppIcon
            v-if="!busy"
            :icon="{ type: 'name', value: isEdit ? 'pencil' : 'plus' }"
            :size="15"
          />
          {{ busy ? "Saving…" : isEdit ? "Save changes" : "Create connection" }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>
