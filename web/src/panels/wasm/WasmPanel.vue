<script setup lang="ts">
import {
  computed,
  nextTick,
  onActivated,
  onDeactivated,
  onMounted,
  onUnmounted,
  ref,
  watch,
} from "vue";
import PanelLoader from "@/components/PanelLoader.vue";
import type { PanelProps } from "@/panels/core/types";
import PanelError from "@/panels/shared/PanelError.vue";
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
const placeholder = ref<HTMLElement | null>(null);
const active = ref(true);
let resizeObserver: ResizeObserver | undefined;
let frame = 0;
let registeredKey: string | undefined;

const stageKey = computed(
  () =>
    props.panelKey ??
    JSON.stringify({
      panel: "wasm",
      connectionId: props.connectionId,
      source: props.source,
      resource: props.resource?.uid,
      config: props.config,
    }),
);
const entry = computed(() =>
  wasmStageEntries.value.find((item) => item.key === stageKey.value),
);
const configError = computed(() =>
  cfg.value ? null : "WASM panel config is required.",
);

watch(
  () => [stageKey.value, props.connectionId, props.resource, props.config],
  () => syncPanel(),
  { deep: true },
);

onMounted(() => {
  resizeObserver = new ResizeObserver(scheduleRectUpdate);
  if (placeholder.value) resizeObserver.observe(placeholder.value);
  window.addEventListener("resize", scheduleRectUpdate);
  window.addEventListener("scroll", scheduleRectUpdate, true);
  syncPanel();
});

onActivated(() => {
  active.value = true;
  syncPanel();
});

onDeactivated(() => {
  active.value = false;
  if (registeredKey) deactivateWasmPanel(registeredKey);
});

onUnmounted(() => {
  resizeObserver?.disconnect();
  window.removeEventListener("resize", scheduleRectUpdate);
  window.removeEventListener("scroll", scheduleRectUpdate, true);
  if (frame) window.cancelAnimationFrame(frame);
  if (registeredKey) unregisterWasmPanel(registeredKey);
});

function syncPanel(): void {
  const config = cfg.value;
  if (!config) {
    if (registeredKey) unregisterWasmPanel(registeredKey);
    registeredKey = undefined;
    return;
  }
  if (registeredKey && registeredKey !== stageKey.value)
    unregisterWasmPanel(registeredKey);
  registeredKey = stageKey.value;
  registerWasmPanel({
    key: registeredKey,
    connectionId: props.connectionId,
    config,
    resource: props.resource,
  });
  if (active.value) activateWasmPanel(registeredKey);
  void nextTick(scheduleRectUpdate);
}

function scheduleRectUpdate(): void {
  if (frame) return;
  frame = window.requestAnimationFrame(() => {
    frame = 0;
    const el = placeholder.value;
    if (!el || !active.value) {
      if (registeredKey) updateWasmPanelRect(registeredKey, null);
      return;
    }
    const rect = el.getBoundingClientRect();
    if (!registeredKey) return;
    updateWasmPanelRect(registeredKey, {
      top: rect.top,
      left: rect.left,
      width: rect.width,
      height: rect.height,
    });
  });
}
</script>

<template>
  <PanelError v-if="configError" :message="configError" />
  <PanelError v-else-if="entry?.error" :message="entry.error" />
  <div
    v-else
    ref="placeholder"
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
