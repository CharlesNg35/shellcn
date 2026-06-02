<script setup lang="ts">
import { computed } from "vue";
import { resolvePanel } from "./registry";
import FallbackPanel from "./FallbackPanel.vue";
import PanelError from "../shared/PanelError.vue";
import { panelConfigError } from "./config";
import { useScopeStore } from "../../stores/scope";
import type {
  Action,
  DataSource,
  PanelType,
  ResourceRef,
  Row,
} from "../../types/projection";

const props = defineProps<{
  panel: PanelType;
  connectionId: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  resource?: ResourceRef | null;
  actions?: Action[];
}>();
const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

const component = computed(() => resolvePanel(props.panel));
const configError = computed(() => panelConfigError(props.panel, props.config));
const scope = useScopeStore();
const panelKey = computed(() =>
  JSON.stringify({
    panel: props.panel,
    connectionId: props.connectionId,
    source: props.source,
    resource: props.resource?.uid,
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
    :source="source"
    :config="config"
    :resource="resource"
    :actions="actions"
    @action-done="onActionDone"
    @select="onSelect"
  />
  <FallbackPanel v-else :panel="panel" />
</template>
