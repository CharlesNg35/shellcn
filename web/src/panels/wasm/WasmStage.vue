<script setup lang="ts">
import { onMounted, onUnmounted, watch } from "vue";
import { useTheme } from "@/composables/useTheme";
import {
  disposeWasmStage,
  onWasmStageMessage,
  refreshWasmStageTheme,
  setWasmPanelIframe,
  wasmStageAllow,
  wasmStageEntries,
  wasmStageEntryStyle,
  wasmStageFrameBoxStyle,
  wasmStageFrameStyle,
  wasmStageSandbox,
  wasmStageViewportClass,
} from "./wasmStage";

const { theme } = useTheme();

onMounted(() => {
  window.addEventListener("message", onWasmStageMessage);
});

onUnmounted(() => {
  window.removeEventListener("message", onWasmStageMessage);
  disposeWasmStage();
});

watch(theme, refreshWasmStageTheme);
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed inset-0 z-30" data-test="wasm-stage">
      <div
        v-for="entry in wasmStageEntries"
        :key="entry.key"
        class="pointer-events-auto bg-surface-0 dark:bg-surface-950"
        :class="wasmStageViewportClass(entry.config)"
        :style="wasmStageEntryStyle(entry)"
        data-test="wasm-stage-entry"
      >
        <div :style="wasmStageFrameBoxStyle(entry)">
          <iframe
            v-if="entry.srcdoc"
            :ref="
              (el) =>
                setWasmPanelIframe(entry.key, el as HTMLIFrameElement | null)
            "
            :aria-label="entry.config.ariaLabel || 'WebAssembly panel'"
            :sandbox="wasmStageSandbox(entry.config)"
            :srcdoc="entry.srcdoc"
            :style="wasmStageFrameStyle(entry)"
            :allow="wasmStageAllow(entry.config)"
            class="block border-0 bg-surface-0 dark:bg-surface-950"
            @load="refreshWasmStageTheme"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>
