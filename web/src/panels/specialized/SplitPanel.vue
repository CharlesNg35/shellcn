<script setup lang="ts">
import { computed } from "vue";
import Splitter from "primevue/splitter";
import SplitterPanel from "primevue/splitterpanel";
import type { PanelProps } from "../core/types";
import type {
  Action,
  Row,
  SplitChildPanel,
  SplitPanelConfig,
} from "@/types/projection";
import { SplitOrientation } from "@/types/projection";
import PanelHost from "../core/PanelHost.vue";
import { resolvedPanelConfig, resolvedPanelType } from "../core/variants";
import { isVisible } from "../form/condition";

const props = defineProps<PanelProps>();
const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

const cfg = computed(
  () => (props.config as SplitPanelConfig | undefined) ?? {},
);
const panels = computed<SplitChildPanel[]>(() => cfg.value.panels ?? []);
const variantData = computed<Record<string, unknown>>(() => {
  if (props.record) {
    return { ...props.record };
  }
  if (props.resource) {
    return { ...props.resource };
  }
  return {};
});
const visiblePanels = computed(() =>
  panels.value.filter((panel) =>
    isVisible(panel.visibleWhen, variantData.value),
  ),
);
const layout = computed(() =>
  cfg.value.orientation === SplitOrientation.Vertical
    ? SplitOrientation.Vertical
    : SplitOrientation.Horizontal,
);
</script>

<template>
  <Splitter class="h-full" :layout="layout">
    <SplitterPanel
      v-for="child in visiblePanels"
      :key="child.key"
      :size="child.size"
      :min-size="child.minSize ?? 10"
      class="min-h-0 overflow-hidden"
    >
      <PanelHost
        :panel="resolvedPanelType(child, variantData)"
        :connection-id="connectionId"
        :source="child.source"
        :config="resolvedPanelConfig(child, variantData)"
        :resource="resource"
        :record="record"
        :actions="actions"
        @action-done="(action, result) => emit('actionDone', action, result)"
        @select="(row) => emit('select', row)"
      />
    </SplitterPanel>
  </Splitter>
</template>
