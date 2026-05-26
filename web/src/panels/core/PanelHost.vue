<script setup lang="ts">
import { computed } from "vue";
import { resolvePanel } from "./registry";
import FallbackPanel from "./FallbackPanel.vue";
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

function onActionDone(action: Action, result?: Record<string, unknown>): void {
  emit("actionDone", action, result);
}

function onSelect(row: Row): void {
  emit("select", row);
}
</script>

<template>
  <component
    :is="component"
    v-if="component"
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
