<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import type { ITheme, Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import type { SearchAddon } from "@xterm/addon-search";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import { useStream } from "../../composables/useStream";
import { useTheme } from "../../composables/useTheme";
import AppIcon from "../../components/AppIcon.vue";
import RecordingControls from "../../components/recordings/RecordingControls.vue";
import type { PanelProps } from "../core/types";
import type { TerminalPanelConfig } from "../../types/projection";
import PanelLoader from "../../components/PanelLoader.vue";
import StreamStatusBar from "./StreamStatusBar.vue";
import type { ChannelStatus } from "../../stores/streamChannels";

const props = withDefaults(
  defineProps<
    PanelProps & {
      streamKeySuffix?: string;
      recordingEnabled?: boolean;
      recordingDisabledReason?: string | null;
    }
  >(),
  {
    streamKeySuffix: undefined,
    recordingEnabled: true,
    recordingDisabledReason: null,
  },
);
const emit = defineEmits<{
  recordingChange: [recording: boolean];
  streamStatusChange: [status: ChannelStatus];
}>();
const { isDark, theme } = useTheme();

const cfg = computed(() => props.config as TerminalPanelConfig | undefined);
const zoomEnabled = computed(() => cfg.value?.zoom === true);
const searchEnabled = computed(() => cfg.value?.search === true);
const hasControls = computed(() => zoomEnabled.value || searchEnabled.value);

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

const recording = computed(() => props.recording ?? null);
const showRecording = computed(
  () => recording.value && recording.value.policy !== "disabled",
);

const container = ref<HTMLElement | null>(null);
const terminalLoading = ref(true);
const failed = ref(false);
const reconnecting = ref(false);
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

const MIN_FONT = 8;
const MAX_FONT = 28;
const DEFAULT_FONT = 14;
const fontSize = ref(DEFAULT_FONT);

function applyFontSize(): void {
  if (!term) return;
  term.options.fontSize = fontSize.value;
  applyFit();
}
function zoomBy(delta: number): void {
  fontSize.value = Math.max(
    MIN_FONT,
    Math.min(MAX_FONT, fontSize.value + delta),
  );
  applyFontSize();
}
function resetZoom(): void {
  fontSize.value = DEFAULT_FONT;
  applyFontSize();
}

let searchAddon: SearchAddon | null = null;
let searchDisposable: { dispose(): void } | null = null;
const searchOpen = ref(false);
const searchTerm = ref("");
const searchInput = ref<{ $el: HTMLElement } | null>(null);
const matches = ref({ current: 0, total: 0 });

const searchDecorations = computed(() => ({
  matchOverviewRuler: "#3b82f6",
  activeMatchColorOverviewRuler: "#f59e0b",
  matchBackground: isDark.value ? "#1d4ed8" : "#bfdbfe",
  activeMatchBackground: "#f59e0b",
}));

function runFind(forward: boolean, incremental = false): void {
  if (!searchAddon) return;
  if (!searchTerm.value) {
    searchAddon.clearDecorations();
    matches.value = { current: 0, total: 0 };
    return;
  }
  const options = { decorations: searchDecorations.value, incremental };
  if (forward) searchAddon.findNext(searchTerm.value, options);
  else searchAddon.findPrevious(searchTerm.value, options);
}

async function openSearch(): Promise<void> {
  if (!searchEnabled.value) return;
  searchOpen.value = true;
  await nextTick();
  const el = searchInput.value?.$el as HTMLElement | undefined;
  const input =
    el instanceof HTMLInputElement ? el : (el?.querySelector("input") ?? null);
  input?.focus();
}
function closeSearch(): void {
  searchOpen.value = false;
  searchTerm.value = "";
  searchAddon?.clearDecorations();
  matches.value = { current: 0, total: 0 };
  term?.focus();
}

watch(searchTerm, () => {
  if (searchOpen.value) runFind(true, true);
});

const { status, error, send, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  write,
  { keySuffix: props.streamKeySuffix },
);

watch(
  status,
  (next) => {
    emit("streamStatusChange", next);
    if (next === "open") {
      lastSize = "";
      void nextTick(applyFit);
    }
  },
  { immediate: true },
);

// Fit the grid to the container; debounced so a burst of resize events settles
// before we re-measure. The resize control frame keeps the remote PTY aligned
// with the local grid size.
function applyFit(): void {
  if (!fit || !term) return;
  try {
    fit.fit();
    if (status.value !== "open") return;
    const size = `${term.cols}x${term.rows}:${theme.value}`;
    if (size !== lastSize) {
      lastSize = size;
      send(
        `\0${JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows, theme: theme.value })}`,
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
  applyFit();
}

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await reconnect();
  } finally {
    reconnecting.value = false;
  }
}

async function mountTerminal(): Promise<void> {
  if (!container.value) {
    terminalLoading.value = false;
    return;
  }
  terminalLoading.value = true;
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
      fontSize: fontSize.value,
      scrollback: 5000,
      // Render an offscreen line for assistive tech (terminals are otherwise opaque).
      screenReaderMode: true,
      theme: { ...terminalTheme.value },
    });
    fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.open(container.value);

    if (searchEnabled.value) {
      const { SearchAddon } = await import("@xterm/addon-search");
      searchAddon = new SearchAddon();
      term.loadAddon(searchAddon);
      searchDisposable = searchAddon.onDidChangeResults(
        ({ resultIndex, resultCount }) => {
          matches.value = {
            current: resultIndex >= 0 ? resultIndex + 1 : 0,
            total: resultCount,
          };
        },
      );
    }

    // Intercept zoom/search shortcuts before the PTY so they don't reach the
    // shell. Only active when the plugin enabled the matching control.
    if (hasControls.value) {
      term.attachCustomKeyEventHandler((e) => {
        if (e.type !== "keydown" || !(e.ctrlKey || e.metaKey)) return true;
        if (searchEnabled.value && (e.key === "f" || e.key === "F")) {
          void openSearch();
          return false;
        }
        if (zoomEnabled.value && (e.key === "=" || e.key === "+")) {
          zoomBy(1);
          return false;
        }
        if (zoomEnabled.value && e.key === "-") {
          zoomBy(-1);
          return false;
        }
        if (zoomEnabled.value && e.key === "0") {
          resetZoom();
          return false;
        }
        return true;
      });
    }

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
  } finally {
    terminalLoading.value = false;
  }
}

onMounted(mountTerminal);
watch(isDark, applyTerminalTheme);

onUnmounted(() => {
  clearTimeout(resizeTimer);
  resizeObserver?.disconnect();
  resizeObserver = null;
  searchDisposable?.dispose();
  searchDisposable = null;
  try {
    term?.dispose();
  } catch {
    /* already disposed */
  }
  term = null;
  fit = null;
  searchAddon = null;
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
        :disabled-reason="recordingEnabled ? null : recordingDisabledReason"
        :stream-status="status"
        @recording-change="emit('recordingChange', $event)"
      />
    </div>
    <StreamStatusBar
      :status="status"
      :error="error"
      :reconnecting="reconnecting"
      can-reconnect
      @reconnect="onReconnect"
    />
    <p v-if="failed" class="p-4 text-base text-surface-400" role="alert">
      Terminal preview unavailable in this environment.
    </p>
    <div class="relative min-h-0 flex-1">
      <PanelLoader
        v-if="terminalLoading && !failed"
        label="Connecting"
        class="absolute inset-0"
      />
      <div
        ref="container"
        role="application"
        aria-label="Terminal session"
        class="absolute inset-0 overflow-hidden"
      />

      <div
        v-if="hasControls"
        class="absolute top-2 right-2 z-10 opacity-60 transition-opacity focus-within:opacity-100 hover:opacity-100"
      >
        <div
          v-if="!searchOpen"
          class="flex items-center gap-0.5 rounded-md border border-surface-200 bg-surface-0/90 p-0.5 shadow-sm backdrop-blur dark:border-surface-700 dark:bg-surface-900/90"
        >
          <template v-if="zoomEnabled">
            <Button
              type="button"
              text
              rounded
              severity="secondary"
              size="small"
              title="Zoom out (Ctrl/⌘ -)"
              aria-label="Zoom out"
              :pt="{ root: 'h-6 w-6 p-0' }"
              @click="zoomBy(-1)"
            >
              <AppIcon :icon="{ type: 'lucide', value: 'minus' }" :size="13" />
            </Button>
            <Button
              type="button"
              text
              severity="secondary"
              size="small"
              title="Reset zoom (Ctrl/⌘ 0)"
              aria-label="Reset zoom"
              :pt="{
                root: 'h-6 min-w-7 px-1',
                label:
                  'text-[10px] tabular-nums text-surface-600 dark:text-surface-300 justify-center items-center flex',
              }"
              :label="`${fontSize}px`"
              @click="resetZoom"
            />
            <Button
              type="button"
              text
              rounded
              severity="secondary"
              size="small"
              title="Zoom in (Ctrl/⌘ +)"
              aria-label="Zoom in"
              :pt="{ root: 'h-6 w-6 p-0' }"
              @click="zoomBy(1)"
            >
              <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="13" />
            </Button>
          </template>
          <Button
            v-if="searchEnabled"
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            title="Search (Ctrl/⌘ F)"
            aria-label="Search terminal"
            :pt="{ root: 'h-6 w-6 p-0' }"
            @click="openSearch"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="13" />
          </Button>
        </div>

        <div
          v-else
          class="flex items-center gap-1 rounded-md border border-surface-200 bg-surface-0/95 px-1.5 py-1 shadow-sm backdrop-blur dark:border-surface-700 dark:bg-surface-900/95"
        >
          <InputText
            ref="searchInput"
            v-model="searchTerm"
            placeholder="Find"
            aria-label="Find in terminal"
            :pt="{ root: 'h-7 w-40 text-xs' }"
            @keydown.enter.exact.prevent="runFind(true)"
            @keydown.shift.enter.prevent="runFind(false)"
            @keydown.esc.prevent="closeSearch"
          />
          <span
            class="min-w-9 text-center text-[10px] text-surface-400 tabular-nums"
          >
            {{ matches.total ? `${matches.current}/${matches.total}` : "0/0" }}
          </span>
          <Button
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            title="Previous (Shift+Enter)"
            aria-label="Previous match"
            :pt="{ root: 'h-6 w-6 p-0' }"
            @click="runFind(false)"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'chevron-up' }"
              :size="14"
            />
          </Button>
          <Button
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            title="Next (Enter)"
            aria-label="Next match"
            :pt="{ root: 'h-6 w-6 p-0' }"
            @click="runFind(true)"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'chevron-down' }"
              :size="14"
            />
          </Button>
          <Button
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            title="Close (Esc)"
            aria-label="Close search"
            :pt="{ root: 'h-6 w-6 p-0' }"
            @click="closeSearch"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
