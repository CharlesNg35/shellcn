<script setup lang="ts">
import DashboardGrid from "./DashboardGrid.vue";
import type {
  Action,
  DashboardCell,
  Row,
  Tab as TabDef,
} from "@/types/projection";

// Connection-level `dashboard` layout: renders the manifest's connection tabs as
// a grid. A thin adapter over the shared DashboardGrid — a Tab is structurally a
// DashboardCell.
const props = defineProps<{
  connectionId: string;
  tabs: TabDef[];
  actions: Action[];
  resolveConfig: (tab: TabDef) => Record<string, unknown>;
  resolvePanel: (tab: TabDef) => TabDef["panel"];
}>();

const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

function resolveCellConfig(cell: DashboardCell): Record<string, unknown> {
  return props.resolveConfig(cell as TabDef);
}

function resolveCellPanel(cell: DashboardCell): TabDef["panel"] {
  return props.resolvePanel(cell as TabDef);
}
</script>

<template>
  <DashboardGrid
    :connection-id="props.connectionId"
    :cells="props.tabs"
    :actions="props.actions"
    :resolve-config="resolveCellConfig"
    :resolve-panel="resolveCellPanel"
    @action-done="(action, result) => emit('actionDone', action, result)"
    @select="(row) => emit('select', row)"
  />
</template>
