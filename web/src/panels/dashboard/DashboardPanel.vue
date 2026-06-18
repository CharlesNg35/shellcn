<script setup lang="ts">
import { computed } from "vue";
import DashboardGrid from "./DashboardGrid.vue";
import type { PanelProps } from "../core/types";
import type {
  Action,
  DashboardCell,
  DashboardPanelConfig,
  Row,
} from "@/types/projection";

// `dashboard` panel type: a multi-panel grid usable as a detail/connection tab.
// Cells come from the manifest config; rendering is delegated to DashboardGrid.
const props = defineProps<PanelProps>();
const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

const cells = computed<DashboardCell[]>(
  () => (props.config as DashboardPanelConfig | undefined)?.cells ?? [],
);
const variantData = computed<Record<string, unknown>>(() => {
  if (props.record) {
    return { ...props.record };
  }
  if (props.resource) {
    return { ...props.resource };
  }
  return {};
});
</script>

<template>
  <DashboardGrid
    :connection-id="props.connectionId"
    :cells="cells"
    :actions="props.actions ?? []"
    :variant-data="variantData"
    :resource="props.resource"
    :record="props.record"
    @action-done="(action, result) => emit('actionDone', action, result)"
    @select="(row) => emit('select', row)"
  />
</template>
