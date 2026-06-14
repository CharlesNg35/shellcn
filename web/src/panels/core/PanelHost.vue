<script setup lang="ts">
import { computed } from "vue";
import { usePanelRecordingResolver } from "@/panels/core/recording";
import PanelError from "@/panels/shared/PanelError.vue";
import { useScopeStore } from "@/stores/scope";
import type {
  Action,
  DataSource,
  PanelType,
  ResourceRef,
  Row,
} from "@/types/projection";
import type { RecordingDescriptor } from "@/composables/useRecordingControl";
import { resolvePanel } from "./registry";
import FallbackPanel from "./FallbackPanel.vue";
import { panelConfigError, usePanelConfigSchemas } from "./config";

const props = defineProps<{
  panel: PanelType;
  connectionId: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  recording?: RecordingDescriptor | null;
  resource?: ResourceRef | null;
  record?: Row | null;
  actions?: Action[];
}>();
const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

const component = computed(() => resolvePanel(props.panel));
const configSchemas = usePanelConfigSchemas();
const configError = computed(() =>
  panelConfigError(props.panel, props.config, configSchemas.value),
);
const recordingResolver = usePanelRecordingResolver();
const panelRecording = computed(() =>
  props.recording === undefined
    ? recordingResolver(props.source)
    : props.recording,
);
const scope = useScopeStore();
const panelKey = computed(() =>
  JSON.stringify({
    panel: props.panel,
    connectionId: props.connectionId,
    source: props.source,
    resource: props.resource?.uid,
    record: props.record,
    scope: scope.key(props.connectionId),
  }),
);

function onActionDone(action: Action, result?: Record<string, unknown>): void {
  emit("actionDone", action, result);
}

function onSelect(row: Row): void {
  emit("select", row);
}
</script>

<template>
  <PanelError v-if="configError" :message="configError" />
  <component
    :is="component"
    v-else-if="component"
    :key="panelKey"
    :connection-id="connectionId"
    :panel-key="panelKey"
    :source="source"
    :config="config"
    :recording="panelRecording"
    :resource="resource"
    :record="record"
    :actions="actions"
    @action-done="onActionDone"
    @select="onSelect"
  />
  <FallbackPanel v-else :panel="panel" />
</template>
