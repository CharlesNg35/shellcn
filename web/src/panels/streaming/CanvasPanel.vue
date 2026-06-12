<script setup lang="ts">
import {
  computed,
  nextTick,
  onActivated,
  onMounted,
  onUnmounted,
  ref,
  watch,
} from "vue";
import { useStream } from "@/composables/useStream";
import { useTheme } from "@/composables/useTheme";
import PanelLoader from "@/components/PanelLoader.vue";
import type { CanvasPanelConfig } from "@/types/projection";
import type { PanelProps } from "../core/types";
import { parseCanvasFrame } from "./canvas/parser";
import {
  Canvas2DRenderer,
  type CanvasResizeOptions,
  type CanvasScaleMode,
} from "./canvas/renderer2d";
import type { CanvasModifierState, CanvasOutgoingEvent } from "./canvas/types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const cfg = computed(() => props.config as CanvasPanelConfig | undefined);
const panelEl = ref<HTMLElement | null>(null);
const canvasEl = ref<HTMLCanvasElement | null>(null);
const hasRenderedFrame = ref(false);
const frameError = ref("");
const { theme } = useTheme();

const renderer = new Canvas2DRenderer((event) => sendEvent(event));

let resizeObserver: ResizeObserver | undefined;
let capturedRegionId: string | undefined;
let pointerMoveFrame = 0;
let pendingPointerMove: CanvasOutgoingEvent | undefined;
const canvasScrollKeys = new Set([
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "End",
  "Home",
  "PageDown",
  "PageUp",
  " ",
  "Spacebar",
]);

const isInteractive = computed(
  () => cfg.value?.interactive || cfg.value?.keyboard || cfg.value?.pointer,
);
const pointerEnabled = computed(
  () => cfg.value?.pointer ?? isInteractive.value,
);
const keyboardEnabled = computed(
  () => cfg.value?.keyboard ?? isInteractive.value,
);
const resizeEvents = computed(() => cfg.value?.resizeEvents ?? true);
const scaleMode = computed<CanvasScaleMode>(() => {
  if (cfg.value?.scaleMode) return cfg.value.scaleMode;
  return "resize";
});
const scrollable = computed(() => scaleMode.value === "scroll");
const wheelEnabled = computed(() => isInteractive.value && !scrollable.value);
const wheelMode = computed(() => cfg.value?.wheelMode || "auto");
const canvasRole = computed(() =>
  isInteractive.value ? "application" : "img",
);
const viewportClass = computed(() => {
  switch (scaleMode.value) {
    case "fit":
      return "grid place-items-center overflow-hidden";
    case "scroll":
      return "overflow-auto overscroll-contain";
    default:
      return "overflow-hidden";
  }
});
const canvasAriaLabel = computed(
  () =>
    cfg.value?.ariaLabel ||
    (isInteractive.value ? "Interactive canvas panel" : "Canvas visualization"),
);
const { status, error, send, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  onFrame,
);
const showInitialLoader = computed(
  () =>
    !hasRenderedFrame.value &&
    !frameError.value &&
    status.value === "connecting",
);
const showEmptyState = computed(
  () =>
    !hasRenderedFrame.value &&
    !frameError.value &&
    status.value !== "connecting",
);

function setupCanvas(): void {
  const canvas = canvasEl.value;
  if (!canvas) return;
  renderer.attach(canvas);
  resizeCanvas();
}

function resizeCanvas(): void {
  const parent = panelEl.value;
  if (!parent) return;
  const size = renderer.resize(
    parent,
    cfg.value?.background,
    cfg.value?.hidpi !== false,
    canvasResizeOptions(),
  );
  if (resizeEvents.value)
    sendEvent({ type: "resize", ...size, theme: theme.value });
}

function canvasResizeOptions(): CanvasResizeOptions {
  return {
    mode: scaleMode.value,
    width: cfg.value?.width,
    height: cfg.value?.height,
    minScale: cfg.value?.minScale,
    maxScale: cfg.value?.maxScale,
  };
}

function onFrame(frame: string): void {
  try {
    renderer.render(parseCanvasFrame(frame), cfg.value?.background);
    hasRenderedFrame.value = true;
    frameError.value = "";
  } catch (err) {
    frameError.value = `Invalid canvas frame: ${(err as Error).message}`;
  }
}

function onPointer(ev: MouseEvent | PointerEvent): void {
  if (!pointerEnabled.value) return;
  ev.preventDefault();
  if (cfg.value?.focusOnPointer ?? true) canvasEl.value?.focus();
  const point = renderer.pointFromEvent(ev);
  const region = capturedRegionId
    ? renderer
        .currentRegions()
        .find((candidate) => candidate.id === capturedRegionId)
    : renderer.regionAt(point);
  if (canvasEl.value) canvasEl.value.style.cursor = region?.cursor || "default";
  if (
    "pointerId" in ev &&
    region?.capturePointer &&
    ev.type === "pointerdown"
  ) {
    canvasEl.value?.setPointerCapture?.(ev.pointerId);
    capturedRegionId = region.id;
  }
  if (
    "pointerId" in ev &&
    (ev.type === "pointerup" || ev.type === "pointercancel")
  ) {
    canvasEl.value?.releasePointerCapture?.(ev.pointerId);
    capturedRegionId = undefined;
  }
  const event: CanvasOutgoingEvent = {
    type: "pointer",
    event: ev.type,
    x: point.x,
    y: point.y,
    button: ev.button,
    buttons: ev.buttons,
    pointerId: "pointerId" in ev ? ev.pointerId : undefined,
    pointerType: "pointerType" in ev ? ev.pointerType : undefined,
    regionId: region?.id,
    modifiers: modifiers(ev),
  };
  if (ev.type === "pointermove") {
    pendingPointerMove = event;
    if (!pointerMoveFrame)
      pointerMoveFrame = requestAnimationFrame(flushPointerMove);
    return;
  }
  flushPointerMove();
  sendEvent(event);
}

function flushPointerMove(): void {
  if (pointerMoveFrame) {
    cancelAnimationFrame(pointerMoveFrame);
    pointerMoveFrame = 0;
  }
  const event = pendingPointerMove;
  pendingPointerMove = undefined;
  if (event) sendEvent(event);
}

function onWheel(ev: WheelEvent): void {
  if (!shouldSendWheel(ev)) return;
  ev.preventDefault();
  const point = renderer.pointFromEvent(ev);
  sendEvent({
    type: "wheel",
    x: point.x,
    y: point.y,
    deltaX: ev.deltaX,
    deltaY: ev.deltaY,
    deltaMode: ev.deltaMode,
    modifiers: modifiers(ev),
  });
}

function shouldSendWheel(ev: WheelEvent): boolean {
  switch (wheelMode.value) {
    case "none":
      return false;
    case "capture":
      return true;
    case "modified":
      return ev.altKey || ev.ctrlKey || ev.metaKey;
    default:
      return Boolean(wheelEnabled.value);
  }
}

function onKey(ev: KeyboardEvent): void {
  if (!keyboardEnabled.value) return;
  if (shouldPreventCanvasKeyDefault(ev)) ev.preventDefault();
  sendEvent({
    type: "key",
    event: ev.type,
    key: ev.key,
    code: ev.code,
    repeat: ev.repeat,
    modifiers: modifiers(ev),
  });
}

function shouldPreventCanvasKeyDefault(ev: KeyboardEvent): boolean {
  if (ev.type !== "keydown") return false;
  if (ev.altKey || ev.ctrlKey || ev.metaKey) return false;
  return canvasScrollKeys.has(ev.key) || canvasScrollKeys.has(ev.code);
}

function sendEvent(event: CanvasOutgoingEvent): void {
  send(JSON.stringify(event));
}

function modifiers(
  ev: MouseEvent | KeyboardEvent | WheelEvent,
): CanvasModifierState {
  return {
    alt: ev.altKey,
    ctrl: ev.ctrlKey,
    meta: ev.metaKey,
    shift: ev.shiftKey,
  };
}

onMounted(() => {
  setupCanvas();
  if (panelEl.value) {
    resizeObserver = new ResizeObserver(() => resizeCanvas());
    resizeObserver.observe(panelEl.value);
  }
});

onActivated(() => resizeCanvas());

onUnmounted(() => {
  resizeObserver?.disconnect();
  flushPointerMove();
});

watch(status, (next) => {
  if (next === "open")
    void nextTick(() =>
      sendEvent({ type: "ready", ...renderer.size(), theme: theme.value }),
    );
});

watch(theme, () => {
  if (status.value === "open") resizeCanvas();
});
</script>

<template>
  <div class="flex h-full min-h-0 flex-col bg-surface-0 dark:bg-surface-950">
    <StreamStatusBar
      :status="status"
      :error="error"
      can-reconnect
      @reconnect="reconnect"
    />
    <div
      ref="panelEl"
      data-test="canvas-panel-viewport"
      class="relative min-h-0 flex-1"
      :class="viewportClass"
      :style="{ background: cfg?.background || 'transparent' }"
    >
      <canvas
        ref="canvasEl"
        data-test="canvas-panel-canvas"
        class="block size-full outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
        :tabindex="keyboardEnabled ? 0 : -1"
        :aria-label="canvasAriaLabel"
        :role="canvasRole"
        @pointerdown="onPointer"
        @pointerup="onPointer"
        @pointermove="onPointer"
        @pointercancel="onPointer"
        @wheel="onWheel"
        @keydown="onKey"
        @keyup="onKey"
      />
      <p class="sr-only">
        {{
          cfg?.instructions ||
          "This panel is controlled by the active plugin. Use keyboard and pointer input when available."
        }}
      </p>
      <PanelLoader v-if="showInitialLoader" class="absolute inset-0" />
      <div
        v-else-if="showEmptyState"
        class="pointer-events-none absolute inset-0 grid place-items-center p-4 text-sm text-surface-500"
      >
        No canvas frames yet.
      </div>
      <div
        v-else-if="frameError"
        class="pointer-events-none absolute inset-0 grid place-items-center p-4 text-sm text-surface-500"
      >
        {{ frameError }}
      </div>
    </div>
  </div>
</template>
