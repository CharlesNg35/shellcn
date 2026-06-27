<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { registerConnectionCleanup } from "@/stores/connectionCleanup";
import {
  disposeWebProxyConnection,
  disposeWebProxyStage,
  markWebProxyPanelLoaded,
  webProxyAllowPolicy,
  webProxyAriaLabel,
  webProxySandboxPolicy,
  webProxyStageEntries,
  webProxyStageEntryStyle,
} from "./webProxyStage";

let unregisterConnectionCleanup: (() => void) | undefined;

onMounted(() => {
  unregisterConnectionCleanup = registerConnectionCleanup(
    disposeWebProxyConnection,
  );
});

onUnmounted(() => {
  unregisterConnectionCleanup?.();
  unregisterConnectionCleanup = undefined;
  disposeWebProxyStage();
});
</script>

<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed inset-0 z-30"
      data-test="web-proxy-stage"
    >
      <div
        v-for="entry in webProxyStageEntries"
        :key="entry.key"
        class="pointer-events-auto overflow-hidden bg-surface-0 dark:bg-surface-950"
        :style="webProxyStageEntryStyle(entry)"
        data-test="web-proxy-stage-entry"
      >
        <iframe
          v-if="entry.src"
          :key="`${entry.key}:${entry.reloadToken}`"
          class="block h-full w-full border-0 bg-white dark:bg-surface-950"
          :src="entry.src"
          :title="webProxyAriaLabel(entry.config)"
          :aria-label="webProxyAriaLabel(entry.config)"
          :sandbox="webProxySandboxPolicy(entry.config)"
          :allow="webProxyAllowPolicy(entry.config)"
          referrerpolicy="no-referrer"
          @load="markWebProxyPanelLoaded(entry.key)"
        />
      </div>
    </div>
  </Teleport>
</template>
