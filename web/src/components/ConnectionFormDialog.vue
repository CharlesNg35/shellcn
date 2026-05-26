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
import ProtocolPicker from "./ProtocolPicker.vue";
import AppIcon from "./AppIcon.vue";
import { dialogRoot, btnPrimary, btnGhost } from "../primevue/preset";
import type {
  ConnectionDetail,
  ConnectionSummary,
  PluginProjection,
  RecordingClass,
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
const selectedPlugin = computed(
  () => conns.plugins.find((p) => p.name === protocol.value) ?? null,
);
const projection = ref<PluginProjection | null>(null);
const name = ref("");
const nameError = ref<string | null>(null);
const transport = ref<Transport>("direct");
const configModel = ref<Record<string, unknown>>({});
const secretsSet = ref<Record<string, boolean>>({});
const recordingModel = ref<Record<string, string>>({});
const loading = ref(false);
const busy = ref(false);
const formRef = ref<{ submit: () => void } | null>(null);

const transportChoices = computed(() =>
  (projection.value?.supportedTransports ?? ["direct"]).map((t) => ({
    label: t === "agent" ? "Agent" : "Direct",
    value: t,
  })),
);

// Shown only when the plugin declares support; never inferred from a panel type.
const recordingClasses = computed(() => projection.value?.recording ?? []);
const recordingLabels: Record<RecordingClass, string> = {
  terminal: "Terminal session",
  desktop: "Desktop / screen",
};
const policyChoices = [
  { label: "Off", value: "disabled" },
  { label: "On demand", value: "manual" },
  { label: "Always", value: "auto" },
];
function recordingLabel(c: RecordingClass): string {
  return recordingLabels[c] ?? c;
}
function policyFor(c: RecordingClass): string {
  return recordingModel.value[c] ?? "disabled";
}
function setPolicy(c: RecordingClass, value: string): void {
  recordingModel.value = { ...recordingModel.value, [c]: value };
}

function reset(): void {
  protocol.value = "";
  projection.value = null;
  name.value = "";
  nameError.value = null;
  transport.value = "direct";
  configModel.value = {};
  secretsSet.value = {};
  recordingModel.value = {};
}

async function selectPlugin(nextProtocol: string): Promise<void> {
  protocol.value = nextProtocol;
  projection.value = await conns.projection(nextProtocol);
  transport.value = projection.value.supportedTransports[0] ?? "direct";
  recordingModel.value = {};
}

// Return to the protocol picker (the breadcrumb "back"), discarding the
// protocol-specific config but keeping the name the user may have typed.
function clearProtocol(): void {
  protocol.value = "";
  projection.value = null;
  configModel.value = {};
  secretsSet.value = {};
  recordingModel.value = {};
  nameError.value = null;
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
    recordingModel.value = { ...(detail.recording ?? {}) };
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
        {
          name: name.value.trim(),
          transport: transport.value,
          config,
          recording: recordingModel.value,
        },
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
        recording: recordingModel.value,
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
      root: dialogRoot('max-w-lg'),
      content: 'max-h-[70vh] overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <p v-if="loading" class="py-8 text-center text-sm text-surface-400">
      Loading…
    </p>

    <div v-else class="flex min-w-0 flex-col gap-5">
      <!-- Step 1: pick a protocol (create only, until one is chosen). -->
      <div v-if="!isEdit && !protocol" class="flex min-w-0 flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Protocol <span class="text-red-500">*</span>
        </label>
        <ProtocolPicker
          :model-value="protocol"
          :plugins="conns.plugins"
          @update:model-value="selectPlugin"
        />
      </div>

      <!-- Loading the chosen protocol's configuration. -->
      <div
        v-else-if="!projection"
        class="flex items-center justify-center gap-2 py-12 text-sm text-surface-400"
      >
        <span
          class="h-4 w-4 animate-spin rounded-full border-2 border-surface-200 border-t-primary-500 dark:border-surface-800 dark:border-t-primary-500"
          role="status"
          aria-label="Loading"
        />
        Loading configuration…
      </div>

      <template v-if="projection">
        <!-- Breadcrumb: the chosen protocol, with a way back to the picker. -->
        <nav aria-label="Breadcrumb" class="flex items-center gap-1.5 text-sm">
          <Button v-if="!isEdit" link @click="clearProtocol">
            Protocols
          </Button>
          <span v-if="!isEdit" class="text-surface-400" aria-hidden="true"
            >/</span
          >
          <span
            class="inline-flex items-center gap-1.5 font-medium text-surface-900 dark:text-surface-100"
          >
            <AppIcon
              :icon="selectedPlugin?.icon ?? projection.icon"
              :size="15"
            />
            {{ selectedPlugin?.title ?? projection.title }}
          </span>
        </nav>

        <div class="flex min-w-0 flex-col gap-1.5">
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

        <div
          v-if="transportChoices.length > 1"
          class="flex min-w-0 flex-col gap-1.5"
        >
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

        <fieldset
          v-if="recordingClasses.length"
          class="flex min-w-0 flex-col gap-3 rounded-md border border-surface-200 p-3 dark:border-surface-700"
        >
          <legend
            class="flex items-center gap-1.5 px-1 text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            <AppIcon :icon="{ type: 'name', value: 'video' }" :size="14" />
            Recording Policy
          </legend>
          <div
            v-for="cap in recordingClasses"
            :key="cap.class"
            class="flex items-center justify-between gap-3"
          >
            <div class="flex min-w-0 flex-col">
              <span class="text-sm text-surface-700 dark:text-surface-200">{{
                recordingLabel(cap.class)
              }}</span>
              <span
                v-if="!cap.authoritative"
                class="text-xs text-amber-600 dark:text-amber-400"
              >
                Browser capture. Not compliance-grade.
              </span>
            </div>
            <div class="w-36 shrink-0">
              <Select
                :model-value="policyFor(cap.class)"
                :options="policyChoices"
                option-label="label"
                option-value="value"
                :aria-label="`Recording for ${recordingLabel(cap.class)}`"
                @update:model-value="setPolicy(cap.class, $event)"
              />
            </div>
          </div>
        </fieldset>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{ root: btnGhost }"
          @click="close"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :disabled="busy || !projection"
          :pt="{ root: btnPrimary }"
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
