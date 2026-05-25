<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { prepareStream } from "../../api/dataSource";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

const loaded = ref(false);
const status = ref("connecting");
const container = ref<HTMLElement | null>(null);
let rfb: { disconnect?: () => void } | null = null;

onMounted(async () => {
  try {
    if (!props.source || !container.value) {
      status.value = "missing-route";
      return;
    }
    const mod = await import("@novnc/novnc");
    const RFB = mod.default as new (
      target: HTMLElement,
      url: string,
      opts?: Record<string, unknown>,
    ) => { disconnect?: () => void };
    const stream = await prepareStream(props.connectionId, props.source, {
      resource: props.resource,
    });
    rfb = new RFB(container.value, stream.url, {
      shared: true,
      repeaterID: props.config?.repeaterID,
    });
    status.value = "ready";
    loaded.value = true;
  } catch (e) {
    status.value = (e as Error).message || "unavailable";
    loaded.value = false;
  }
});

onUnmounted(() => {
  rfb?.disconnect?.();
  rfb = null;
  loaded.value = false;
});
</script>

<template>
  <div class="flex h-full flex-col bg-black">
    <StubBanner :status="loaded ? 'ready' : status" />
    <div ref="container" class="min-h-0 flex-1">
      <p v-if="!loaded" class="p-4 text-sm text-surface-400">
        Remote desktop session is waiting for a VNC route.
      </p>
    </div>
  </div>
</template>
