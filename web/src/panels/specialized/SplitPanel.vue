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

const props = defineProps<PanelProps>();
const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

const cfg = computed(
  () => (props.config as SplitPanelConfig | undefined) ?? {},
);
const panels = computed<SplitChildPanel[]>(() => cfg.value.panels ?? []);
const variantData = computed<Record<string, unknown>>(() =>
  props.resource ? { ...props.resource } : {},
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
      v-for="child in panels"
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
        :actions="actions"
        @action-done="(action, result) => emit('actionDone', action, result)"
        @select="(row) => emit('select', row)"
      />
    </SplitterPanel>
  </Splitter>
</template>
