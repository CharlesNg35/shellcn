<script setup lang="ts">
import { computed } from "vue";
import DashboardGrid from "./DashboardGrid.vue";
import type { PanelProps } from "../core/types";
import type {
  Action,
  DashboardCell,
  DashboardPanelConfig,
} from "../../types/projection";

// `dashboard` panel type: a multi-panel grid usable as a detail/connection tab.
// Cells come from the manifest config; rendering is delegated to DashboardGrid.
const props = defineProps<PanelProps>();
const emit = defineEmits<{ actionDone: [action: Action] }>();

const cells = computed<DashboardCell[]>(
  () => (props.config as DashboardPanelConfig | undefined)?.cells ?? [],
);
</script>

<template>
  <DashboardGrid
    :connection-id="props.connectionId"
    :cells="cells"
    :actions="props.actions ?? []"
    @action-done="(action) => emit('actionDone', action)"
  />
</template>
