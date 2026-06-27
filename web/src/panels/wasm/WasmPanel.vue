<script setup lang="ts">
import { computed } from "vue";
import PanelLoader from "@/components/PanelLoader.vue";
import type { PanelProps } from "@/panels/core/types";
import PanelError from "@/panels/shared/PanelError.vue";
import { usePersistentStagePanel } from "@/panels/shared/usePersistentStagePanel";
import type { WasmPanelConfig } from "@/types/projection";
import {
  activateWasmPanel,
  deactivateWasmPanel,
  registerWasmPanel,
  unregisterWasmPanel,
  updateWasmPanelRect,
  wasmStageEntries,
} from "./wasmStage";

const props = defineProps<PanelProps>();

const cfg = computed(() => props.config as WasmPanelConfig | undefined);
const stageKey = computed(
  () =>
    props.panelKey ??
    JSON.stringify({
      panel: "wasm",
      connectionId: props.connectionId,
      source: props.source,
      resource: props.resource?.uid,
      record: props.record,
      config: props.config,
    }),
);
const entry = computed(() =>
  wasmStageEntries.value.find((item) => item.key === stageKey.value),
);
const configError = computed(() =>
  cfg.value ? null : "WASM panel config is required.",
);
const handle = computed(() =>
  cfg.value
    ? {
        key: stageKey.value,
        connectionId: props.connectionId,
        config: cfg.value,
        resource: props.resource,
        record: props.record,
      }
    : null,
);

const { setPlaceholder } = usePersistentStagePanel({
  stageKey,
  handle,
  watchSource: () => [
    stageKey.value,
    props.connectionId,
    props.resource,
    props.record,
    props.config,
  ],
  deep: true,
  register: registerWasmPanel,
  activate: activateWasmPanel,
  deactivate: deactivateWasmPanel,
  unregister: unregisterWasmPanel,
  updateRect: updateWasmPanelRect,
});
</script>

<template>
  <PanelError v-if="configError" :message="configError" />
  <PanelError v-else-if="entry?.error" :message="entry.error" />
  <div
    v-else
    :ref="setPlaceholder"
    class="relative h-full min-h-0 overflow-hidden bg-surface-0 dark:bg-surface-950"
    data-test="wasm-panel-placeholder"
  >
    <PanelLoader
      v-if="entry?.loading || !entry?.srcdoc"
      class="absolute inset-0 bg-surface-0 dark:bg-surface-950"
    />
    <p class="sr-only">
      {{
        cfg?.instructions ||
        "This panel runs a sandboxed WebAssembly app declared by the active plugin."
      }}
    </p>
  </div>
</template>
