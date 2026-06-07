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
import { useStream } from "../../composables/useStream";
import type { CanvasPanelConfig } from "../../types/projection";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

type CanvasPoint = { x: number; y: number };
type CanvasRegion = {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  cursor?: string;
  label?: string;
};
type CanvasCommand = Record<string, unknown> & { type?: string };
type CanvasFrame = CanvasCommand & {
  commands?: CanvasCommand[];
  regions?: CanvasRegion[];
};

const cfg = computed(() => props.config as CanvasPanelConfig | undefined);
const panelEl = ref<HTMLElement | null>(null);
const canvasEl = ref<HTMLCanvasElement | null>(null);
const statusText = ref("Waiting for canvas frames...");
const regions = ref<CanvasRegion[]>([]);
const imageCache = new Map<string, HTMLImageElement>();

let ctx: CanvasRenderingContext2D | null = null;
let logicalWidth = cfg.value?.width || 800;
let logicalHeight = cfg.value?.height || 450;
let dpr = 1;

const isInteractive = computed(
  () => cfg.value?.interactive || cfg.value?.keyboard || cfg.value?.pointer,
);
const pointerEnabled = computed(
  () => cfg.value?.pointer ?? isInteractive.value,
);
const keyboardEnabled = computed(
  () => cfg.value?.keyboard ?? isInteractive.value,
);
const wheelEnabled = computed(() => cfg.value?.wheel ?? isInteractive.value);
const resizeEvents = computed(() => cfg.value?.resizeEvents ?? true);

const { status, error, send, reconnect } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  onFrame,
);

function setupCanvas(): void {
  const canvas = canvasEl.value;
  if (!canvas) return;
  ctx = canvas.getContext("2d");
  resizeCanvas();
}

function resizeCanvas(): void {
  const canvas = canvasEl.value;
  const parent = panelEl.value;
  if (!canvas || !parent) return;
  const rect = parent.getBoundingClientRect();
  logicalWidth = Math.max(1, Math.round(rect.width || cfg.value?.width || 800));
  logicalHeight = Math.max(
    1,
    Math.round(rect.height || cfg.value?.height || 450),
  );
  dpr = cfg.value?.hidpi === false ? 1 : window.devicePixelRatio || 1;
  canvas.width = Math.round(logicalWidth * dpr);
  canvas.height = Math.round(logicalHeight * dpr);
  canvas.style.width = `${logicalWidth}px`;
  canvas.style.height = `${logicalHeight}px`;
  ctx = canvas.getContext("2d");
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0);
  clearCanvas(cfg.value?.background);
  if (resizeEvents.value) sendEvent("resize", baseEvent());
}

function onFrame(frame: string): void {
  try {
    const parsed = JSON.parse(frame) as CanvasFrame | CanvasCommand[];
    if (Array.isArray(parsed)) {
      runCommands(parsed);
    } else if (Array.isArray(parsed.commands)) {
      runCommands(parsed.commands);
      if (Array.isArray(parsed.regions)) regions.value = parsed.regions;
    } else {
      runCommand(parsed);
    }
    statusText.value = "";
  } catch (err) {
    statusText.value = `Invalid canvas frame: ${(err as Error).message}`;
  }
}

function runCommands(commands: CanvasCommand[]): void {
  for (const command of commands) runCommand(command);
}

function runCommand(command: CanvasCommand): void {
  if (!ctx || !command.type) return;
  switch (command.type) {
    case "clear":
      clearCanvas(str(command.color) || cfg.value?.background);
      break;
    case "set":
      if (typeof command.background === "string")
        clearCanvas(command.background);
      if (Array.isArray(command.regions))
        regions.value = command.regions as CanvasRegion[];
      if (typeof command.cursor === "string" && canvasEl.value)
        canvasEl.value.style.cursor = command.cursor;
      break;
    case "regions":
      regions.value = Array.isArray(command.items)
        ? (command.items as CanvasRegion[])
        : [];
      break;
    case "save":
      ctx.save();
      break;
    case "restore":
      ctx.restore();
      break;
    case "resetTransform":
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      break;
    case "translate":
      ctx.translate(num(command.x), num(command.y));
      break;
    case "scale":
      ctx.scale(num(command.x, 1), num(command.y, num(command.x, 1)));
      break;
    case "rotate":
      ctx.rotate(num(command.angle));
      break;
    case "style":
      applyStyle(command);
      break;
    case "rect":
      drawRect(command);
      break;
    case "line":
      drawLine(command);
      break;
    case "polyline":
    case "polygon":
      drawPolyline(command, command.type === "polygon");
      break;
    case "circle":
      drawCircle(command);
      break;
    case "ellipse":
      drawEllipse(command);
      break;
    case "path":
      drawPath(command);
      break;
    case "text":
      drawText(command);
      break;
    case "image":
      drawImage(command);
      break;
  }
}

function clearCanvas(color?: string): void {
  if (!ctx) return;
  ctx.save();
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, logicalWidth, logicalHeight);
  if (color) {
    ctx.fillStyle = color;
    ctx.fillRect(0, 0, logicalWidth, logicalHeight);
  }
  ctx.restore();
}

function applyStyle(command: CanvasCommand): void {
  if (!ctx) return;
  if (typeof command.fill === "string") ctx.fillStyle = command.fill;
  if (typeof command.stroke === "string") ctx.strokeStyle = command.stroke;
  if (typeof command.lineWidth === "number") ctx.lineWidth = command.lineWidth;
  if (typeof command.font === "string") ctx.font = command.font;
  if (typeof command.alpha === "number") ctx.globalAlpha = command.alpha;
  if (typeof command.composite === "string")
    ctx.globalCompositeOperation =
      command.composite as GlobalCompositeOperation;
  if (typeof command.lineCap === "string")
    ctx.lineCap = command.lineCap as CanvasLineCap;
  if (typeof command.lineJoin === "string")
    ctx.lineJoin = command.lineJoin as CanvasLineJoin;
  if (typeof command.textAlign === "string")
    ctx.textAlign = command.textAlign as CanvasTextAlign;
  if (typeof command.textBaseline === "string")
    ctx.textBaseline = command.textBaseline as CanvasTextBaseline;
}

function drawRect(command: CanvasCommand): void {
  if (!ctx) return;
  applyStyle(command);
  const x = num(command.x);
  const y = num(command.y);
  const w = num(command.width);
  const h = num(command.height);
  const r = num(command.radius);
  ctx.beginPath();
  if (r > 0 && "roundRect" in ctx) ctx.roundRect(x, y, w, h, r);
  else ctx.rect(x, y, w, h);
  fillStroke(command);
}

function drawLine(command: CanvasCommand): void {
  if (!ctx) return;
  applyStyle(command);
  ctx.beginPath();
  ctx.moveTo(num(command.x1), num(command.y1));
  ctx.lineTo(num(command.x2), num(command.y2));
  ctx.stroke();
}

function drawPolyline(command: CanvasCommand, close: boolean): void {
  if (!ctx || !Array.isArray(command.points)) return;
  const points = command.points as CanvasPoint[];
  if (!points.length) return;
  applyStyle(command);
  ctx.beginPath();
  ctx.moveTo(num(points[0].x), num(points[0].y));
  for (const p of points.slice(1)) ctx.lineTo(num(p.x), num(p.y));
  if (close) ctx.closePath();
  fillStroke(command, !close);
}

function drawCircle(command: CanvasCommand): void {
  if (!ctx) return;
  applyStyle(command);
  ctx.beginPath();
  ctx.arc(num(command.x), num(command.y), num(command.radius), 0, Math.PI * 2);
  fillStroke(command);
}

function drawEllipse(command: CanvasCommand): void {
  if (!ctx) return;
  applyStyle(command);
  ctx.beginPath();
  ctx.ellipse(
    num(command.x),
    num(command.y),
    num(command.radiusX),
    num(command.radiusY),
    num(command.rotation),
    0,
    Math.PI * 2,
  );
  fillStroke(command);
}

function drawPath(command: CanvasCommand): void {
  if (!ctx || typeof command.d !== "string") return;
  applyStyle(command);
  const path = new Path2D(command.d);
  if (command.fill !== false) ctx.fill(path);
  if (command.stroke !== false) ctx.stroke(path);
}

function drawText(command: CanvasCommand): void {
  if (!ctx) return;
  ctx.textAlign = "start";
  ctx.textBaseline = "alphabetic";
  applyStyle(command);
  const text = str(command.text);
  const x = num(command.x);
  const y = num(command.y);
  const maxWidth =
    typeof command.maxWidth === "number" ? command.maxWidth : undefined;
  if (command.stroke) ctx.strokeText(text, x, y, maxWidth);
  if (command.fill !== false) ctx.fillText(text, x, y, maxWidth);
}

function drawImage(command: CanvasCommand): void {
  if (!ctx || typeof command.src !== "string") return;
  const src = command.src;
  let image = imageCache.get(src);
  if (!image) {
    image = new Image();
    image.crossOrigin = "anonymous";
    image.onload = () => drawImage(command);
    image.src = src;
    imageCache.set(src, image);
    return;
  }
  if (!image.complete) return;
  ctx.save();
  applyStyle(command);
  ctx.globalAlpha = num(command.alpha, ctx.globalAlpha);
  ctx.drawImage(
    image,
    num(command.x),
    num(command.y),
    num(command.width, image.naturalWidth || image.width),
    num(command.height, image.naturalHeight || image.height),
  );
  ctx.restore();
}

function fillStroke(command: CanvasCommand, strokeDefault = false): void {
  if (!ctx) return;
  if (command.fill !== false) ctx.fill();
  if (command.stroke !== false && (strokeDefault || command.stroke))
    ctx.stroke();
}

function canvasPoint(ev: MouseEvent | WheelEvent): CanvasPoint {
  const canvas = canvasEl.value;
  if (!canvas) return { x: 0, y: 0 };
  const rect = canvas.getBoundingClientRect();
  return {
    x: ((ev.clientX - rect.left) / Math.max(1, rect.width)) * logicalWidth,
    y: ((ev.clientY - rect.top) / Math.max(1, rect.height)) * logicalHeight,
  };
}

function regionAt(point: CanvasPoint): CanvasRegion | undefined {
  return regions.value
    .slice()
    .reverse()
    .find(
      (r) =>
        point.x >= r.x &&
        point.x <= r.x + r.width &&
        point.y >= r.y &&
        point.y <= r.y + r.height,
    );
}

function onPointer(ev: MouseEvent | PointerEvent): void {
  if (!pointerEnabled.value) return;
  if (cfg.value?.focusOnPointer ?? true) canvasEl.value?.focus();
  const point = canvasPoint(ev);
  const region = regionAt(point);
  if (canvasEl.value) canvasEl.value.style.cursor = region?.cursor || "default";
  sendEvent("pointer", {
    event: ev.type,
    x: point.x,
    y: point.y,
    button: ev.button,
    buttons: ev.buttons,
    pointerId: "pointerId" in ev ? ev.pointerId : undefined,
    pointerType: "pointerType" in ev ? ev.pointerType : undefined,
    regionId: region?.id,
    modifiers: modifiers(ev),
  });
}

function onWheel(ev: WheelEvent): void {
  if (!wheelEnabled.value) return;
  ev.preventDefault();
  const point = canvasPoint(ev);
  sendEvent("wheel", {
    x: point.x,
    y: point.y,
    deltaX: ev.deltaX,
    deltaY: ev.deltaY,
    deltaMode: ev.deltaMode,
    modifiers: modifiers(ev),
  });
}

function onKey(ev: KeyboardEvent): void {
  if (!keyboardEnabled.value) return;
  sendEvent("key", {
    event: ev.type,
    key: ev.key,
    code: ev.code,
    repeat: ev.repeat,
    modifiers: modifiers(ev),
  });
}

function baseEvent(): Record<string, unknown> {
  return { width: logicalWidth, height: logicalHeight, dpr };
}

function sendEvent(type: string, payload: Record<string, unknown>): void {
  send(JSON.stringify({ type, ...payload }));
}

function modifiers(
  ev: MouseEvent | KeyboardEvent | WheelEvent,
): Record<string, boolean> {
  return {
    alt: ev.altKey,
    ctrl: ev.ctrlKey,
    meta: ev.metaKey,
    shift: ev.shiftKey,
  };
}

function num(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function str(value: unknown): string {
  return typeof value === "string" ? value : String(value ?? "");
}

let resizeObserver: ResizeObserver | undefined;

onMounted(() => {
  setupCanvas();
  if (panelEl.value) {
    resizeObserver = new ResizeObserver(() => resizeCanvas());
    resizeObserver.observe(panelEl.value);
  }
});

onActivated(() => resizeCanvas());

onUnmounted(() => resizeObserver?.disconnect());

watch(status, (next) => {
  if (next === "open") void nextTick(() => sendEvent("ready", baseEvent()));
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
      class="relative min-h-0 flex-1 overflow-hidden"
      :style="{ background: cfg?.background || 'transparent' }"
    >
      <canvas
        ref="canvasEl"
        data-test="canvas-panel-canvas"
        class="block size-full outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
        :tabindex="keyboardEnabled ? 0 : -1"
        :aria-label="cfg?.ariaLabel || 'Interactive canvas panel'"
        role="application"
        @pointerdown="onPointer"
        @pointerup="onPointer"
        @pointermove="onPointer"
        @pointercancel="onPointer"
        @click="onPointer"
        @dblclick="onPointer"
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
      <div
        v-if="statusText"
        class="pointer-events-none absolute inset-0 grid place-items-center p-4 text-sm text-surface-500"
      >
        {{ statusText }}
      </div>
    </div>
  </div>
</template>
