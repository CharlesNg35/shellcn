<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import type { ITheme, Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { useStream } from "../../composables/useStream";
import { useTheme } from "../../composables/useTheme";
import RecordingControls from "../../components/recordings/RecordingControls.vue";
import type { RecordingDescriptor } from "../../composables/useRecordingControl";
import type { PanelProps } from "../types";

const props = defineProps<PanelProps>();
const { isDark } = useTheme();

const darkTerminalTheme: ITheme = {
  background: "#020617",
  foreground: "#e2e8f0",
  cursor: "#93c5fd",
  cursorAccent: "#020617",
  selectionBackground: "#1d4ed866",
  black: "#020617",
  red: "#ef4444",
  green: "#22c55e",
  yellow: "#eab308",
  blue: "#3b82f6",
  magenta: "#a855f7",
  cyan: "#06b6d4",
  white: "#e2e8f0",
  brightBlack: "#64748b",
  brightRed: "#f87171",
  brightGreen: "#4ade80",
  brightYellow: "#facc15",
  brightBlue: "#60a5fa",
  brightMagenta: "#c084fc",
  brightCyan: "#22d3ee",
  brightWhite: "#f8fafc",
};

const lightTerminalTheme: ITheme = {
  background: "#ffffff",
  foreground: "#334155",
  cursor: "#2563eb",
  cursorAccent: "#ffffff",
  selectionBackground: "#bfdbfe",
  black: "#0f172a",
  red: "#dc2626",
  green: "#16a34a",
  yellow: "#ca8a04",
  blue: "#2563eb",
  magenta: "#9333ea",
  cyan: "#0891b2",
  white: "#f8fafc",
  brightBlack: "#64748b",
  brightRed: "#ef4444",
  brightGreen: "#22c55e",
  brightYellow: "#eab308",
  brightBlue: "#3b82f6",
  brightMagenta: "#a855f7",
  brightCyan: "#06b6d4",
  brightWhite: "#ffffff",
};

const terminalTheme = computed(() =>
  isDark.value ? darkTerminalTheme : lightTerminalTheme,
);
const terminalSurface = computed(() =>
  isDark.value
    ? "bg-surface-950 text-surface-100"
    : "bg-surface-0 text-surface-800",
);

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
// before we re-measure. The resize control frame keeps the remote PTY aligned
// with the local grid size.
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

function applyTerminalTheme(): void {
  if (!term) return;
  term.options.theme = { ...terminalTheme.value };
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
      theme: { ...terminalTheme.value },
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
watch(isDark, applyTerminalTheme);

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
  <div class="flex h-full flex-col" :class="terminalSurface">
    <div
      v-if="showRecording && source"
      class="flex items-center justify-end border-b border-surface-200 px-3 py-1.5 dark:border-white/5"
    >
      <RecordingControls
        :connection-id="connectionId"
        :source="source"
        :resource="resource"
        :descriptor="recording!"
      />
    </div>
    <p
      v-if="status === 'error' || status === 'closed'"
      class="border-b border-red-300/40 bg-red-50 px-3 py-1.5 text-xs text-red-700 dark:bg-red-950/40 dark:text-red-300"
      role="alert"
    >
      Terminal stream {{ status }}.
    </p>
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
