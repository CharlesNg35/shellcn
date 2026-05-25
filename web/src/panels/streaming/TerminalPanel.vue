<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { useStream } from "../../composables/useStream";
import RecordingControls from "../../components/recordings/RecordingControls.vue";
import type { RecordingDescriptor } from "../../composables/useRecordingControl";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

const recording = computed(
  () => (props.config?._recording as RecordingDescriptor | undefined) ?? null,
);
const showRecording = computed(
  () => recording.value && recording.value.policy !== "disabled",
);

const container = ref<HTMLElement | null>(null);
const failed = ref(false);
const pending: string[] = [];
let term: Terminal | null = null;
let fit: FitAddon | null = null;
let resizeObserver: ResizeObserver | null = null;
let resizeTimer: ReturnType<typeof setTimeout> | undefined;
let lastSize = "";

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

// Fit the grid to the container; debounced so a burst of resize events settles
// before we re-measure. Live cols/rows propagation to the PTY is wired by the
// protocol plugin (M2 SSH) — this keeps the local view correct in the meantime.
function applyFit(): void {
  if (!fit || !term) return;
  try {
    fit.fit();
    const size = `${term.cols}x${term.rows}`;
    if (size !== lastSize) {
      lastSize = size;
      send(
        `\0${JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })}`,
      );
    }
  } catch {
    /* container not measurable yet */
  }
}

function scheduleFit(): void {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(applyFit, 100);
}

async function mountTerminal(): Promise<void> {
  if (!container.value) return;
  try {
    const [{ Terminal }, { FitAddon }, { WebLinksAddon }] = await Promise.all([
      import("@xterm/xterm"),
      import("@xterm/addon-fit"),
      import("@xterm/addon-web-links"),
      import("@xterm/xterm/css/xterm.css"),
    ]);
    term = new Terminal({
      convertEol: true,
      cursorBlink: true,
      fontSize: 13,
      scrollback: 5000,
      // Render an offscreen line for assistive tech (terminals are otherwise opaque).
      screenReaderMode: true,
      theme: { background: "#0b0f17" },
    });
    fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.open(container.value);

    // WebGL renderer for fast large-output scrolling; fall back to the DOM
    // renderer (and never throw) if the GPU context is lost or unavailable.
    try {
      const { WebglAddon } = await import("@xterm/addon-webgl");
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => webgl.dispose());
      term.loadAddon(webgl);
    } catch {
      /* no WebGL — DOM renderer is the default fallback */
    }

    applyFit();
    pending.forEach((d) => term!.write(d));
    pending.length = 0;
    term.onData((d: string) => send(d));
    term.focus();

    resizeObserver = new ResizeObserver(scheduleFit);
    resizeObserver.observe(container.value);
  } catch {
    failed.value = true;
  }
}

onMounted(mountTerminal);

onUnmounted(() => {
  clearTimeout(resizeTimer);
  resizeObserver?.disconnect();
  resizeObserver = null;
  try {
    term?.dispose();
  } catch {
    /* already disposed */
  }
  term = null;
  fit = null;
});
</script>

<template>
  <div class="flex h-full flex-col bg-[#0b0f17]">
    <div
      v-if="showRecording && source"
      class="flex items-center justify-end border-b border-white/5 px-3 py-1.5"
    >
      <RecordingControls
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :descriptor="recording!"
      />
    </div>
    <StubBanner :status="status" />
    <p v-if="failed" class="p-4 text-sm text-surface-400" role="alert">
      Terminal preview unavailable in this environment.
    </p>
    <div
      ref="container"
      role="application"
      aria-label="Terminal session"
      class="min-h-0 flex-1 overflow-hidden p-2"
    />
  </div>
</template>
