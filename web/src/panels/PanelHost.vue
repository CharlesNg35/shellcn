<script setup lang="ts">
import { computed } from "vue";
import { resolvePanel } from "./registry";
import FallbackPanel from "./FallbackPanel.vue";
import type {
  Action,
  DataSource,
  PanelType,
  ResourceRef,
} from "../types/projection";

const props = defineProps<{
  panel: PanelType;
  connectionId: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  resource?: ResourceRef | null;
  actions?: Action[];
}>();

const component = computed(() => resolvePanel(props.panel));
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
  />
  <FallbackPanel v-else :panel="panel" />
</template>
