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
}>();

const emit = defineEmits<{
  actionDone: [action: Action];
  select: [row: Row];
}>();

function resolveCellConfig(cell: DashboardCell): Record<string, unknown> {
  return props.resolveConfig(cell as TabDef);
}
</script>

<template>
  <DashboardGrid
    :connection-id="props.connectionId"
    :cells="props.tabs"
    :actions="props.actions"
    :resolve-config="resolveCellConfig"
    @action-done="(action) => emit('actionDone', action)"
    @select="(row) => emit('select', row)"
  />
</template>
