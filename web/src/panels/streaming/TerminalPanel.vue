<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

const container = ref<HTMLElement | null>(null);
const failed = ref(false);
const pending: string[] = [];
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- xterm Terminal type loaded lazily
let term: any = null;

function write(data: string): void {
  if (term) term.write(data);
  else pending.push(data);
}

const { status, send } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  write,
);

async function mountTerminal(): Promise<void> {
  if (!container.value) return;
  try {
    const { Terminal } = await import("@xterm/xterm");
    await import("@xterm/xterm/css/xterm.css");
    term = new Terminal({
      convertEol: true,
      fontSize: 13,
      theme: { background: "#0b0f17" },
    });
    term.open(container.value);
    pending.forEach((d) => term.write(d));
    pending.length = 0;
    term.onData((d: string) => send(d));
  } catch {
    failed.value = true;
  }
}

onMounted(mountTerminal);

onUnmounted(() => {
  try {
    term?.dispose();
  } catch {
    /* already disposed */
  }
});
</script>

<template>
  <div class="flex h-full flex-col bg-[#0b0f17]">
    <StubBanner :status="status" />
    <p v-if="failed" class="p-4 text-sm text-surface-400">
      Terminal preview unavailable in this environment.
    </p>
    <div ref="container" class="min-h-0 flex-1 overflow-hidden p-2" />
  </div>
</template>
