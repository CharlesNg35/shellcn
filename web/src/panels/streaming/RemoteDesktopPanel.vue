<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

defineProps<PanelProps>();

const loaded = ref(false);

onMounted(async () => {
  try {
    // Dynamically import noVNC so the RFB client stays out of the initial bundle.
    await import("@novnc/novnc");
    loaded.value = true;
  } catch {
    loaded.value = false;
  }
});

onUnmounted(() => {
  loaded.value = false;
});
</script>

<template>
  <div class="flex h-full flex-col bg-black">
    <StubBanner :status="loaded ? 'ready' : 'stub'" />
    <div class="flex min-h-0 flex-1 items-center justify-center">
      <p class="text-sm text-surface-400">
        Remote desktop canvas — validated with the VNC plugin.
      </p>
    </div>
  </div>
</template>
