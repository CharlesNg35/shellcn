<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from "vue";
import ToastEventBus from "primevue/toasteventbus";
import AppIcon from "./AppIcon.vue";
import type { Icon } from "../types/projection";

// Sonner-style toaster: a collapsed stack that fans out on hover/focus. We
// subscribe straight to PrimeVue's ToastEventBus (what useToast().add emits on),
// so every existing call site keeps working while we own the rendering entirely.

interface ToastMessage {
  severity?: string;
  summary?: string;
  detail?: string;
  life?: number;
  sticky?: boolean;
}
interface Item {
  id: number;
  severity: string;
  summary?: string;
  detail?: string;
  life: number;
  sticky: boolean;
  mounted: boolean;
  removing: boolean;
}
interface Tone {
  icon: Icon;
  accent: string;
  role: "status" | "alert";
}

const tones: Record<string, Tone> = {
  success: {
    icon: { type: "lucide", value: "circle-check" },
    accent: "text-emerald-500",
    role: "status",
  },
  error: {
    icon: { type: "lucide", value: "circle-alert" },
    accent: "text-rose-500",
    role: "alert",
  },
  warn: {
    icon: { type: "lucide", value: "triangle-alert" },
    accent: "text-amber-500",
    role: "alert",
  },
  info: {
    icon: { type: "lucide", value: "info" },
    accent: "text-sky-500",
    role: "status",
  },
};
function tone(severity: string): Tone {
  return tones[severity] ?? tones.info;
}

const MAX_VISIBLE = 3; // toasts kept fully opaque behind the front one when collapsed
const PEEK = 16; // px each stacked toast peeks above the one in front
const GAP = 14; // px between toasts when expanded
const SCALE_STEP = 0.055; // per-depth shrink when collapsed
const LEAVE_MS = 260; // must match the leave transition in style.css
const DEFAULT_HEIGHT = 64; // fallback until a toast is measured

const items = ref<Item[]>([]); // newest first: index 0 is the front toast
const expanded = ref(false);
const heights = reactive<Record<number, number>>({});
const timers = new Map<number, { start: number; remaining: number; handle: number }>();
let seq = 0;

const cards = new Map<number, HTMLElement>();
let ro: ResizeObserver | undefined;

function setCard(id: number, el: unknown): void {
  const node = el as HTMLElement | null;
  if (node) {
    cards.set(id, node);
    ro?.observe(node);
    heights[id] = node.offsetHeight;
  } else {
    const prev = cards.get(id);
    if (prev) {
      ro?.unobserve(prev);
      cards.delete(id);
    }
  }
}

function add(msg: ToastMessage): void {
  const id = ++seq;
  const item: Item = {
    id,
    severity: msg.severity ?? "info",
    summary: msg.summary,
    detail: msg.detail,
    life: msg.life ?? 3000,
    sticky: Boolean(msg.sticky),
    mounted: false,
    removing: false,
  };
  items.value.unshift(item);
  nextTick(() => {
    const it = items.value.find((x) => x.id === id);
    if (it) it.mounted = true;
  });
  if (!item.sticky && item.life > 0) startTimer(id, item.life);
}

function startTimer(id: number, life: number): void {
  const rec = { start: Date.now(), remaining: life, handle: 0 };
  rec.handle = window.setTimeout(() => dismiss(id), life);
  timers.set(id, rec);
}
function pauseTimers(): void {
  timers.forEach((rec) => {
    window.clearTimeout(rec.handle);
    rec.remaining -= Date.now() - rec.start;
  });
}
function resumeTimers(): void {
  timers.forEach((rec, id) => {
    rec.start = Date.now();
    rec.handle = window.setTimeout(() => dismiss(id), Math.max(0, rec.remaining));
  });
}
function dismiss(id: number): void {
  const it = items.value.find((x) => x.id === id);
  if (!it || it.removing) return;
  it.removing = true;
  const rec = timers.get(id);
  if (rec) window.clearTimeout(rec.handle);
  timers.delete(id);
  window.setTimeout(() => {
    items.value = items.value.filter((x) => x.id !== id);
    delete heights[id];
  }, LEAVE_MS);
}

function onExpand(): void {
  expanded.value = true;
  pauseTimers();
}
function onCollapse(): void {
  expanded.value = false;
  resumeTimers();
}

function heightOf(id: number): number {
  return heights[id] ?? DEFAULT_HEIGHT;
}
function offsetBefore(index: number): number {
  let acc = 0;
  for (let i = 0; i < index; i++) acc += heightOf(items.value[i].id);
  return acc;
}

const totalHeight = computed(() => {
  const list = items.value;
  if (!list.length) return 0;
  let h = 0;
  for (const it of list) h += heightOf(it.id);
  return h + (list.length - 1) * GAP;
});
const frontHeight = computed(() =>
  items.value[0] ? heightOf(items.value[0].id) : 0,
);
const containerStyle = computed(() => ({
  height: `${expanded.value ? totalHeight.value : frontHeight.value}px`,
}));

function cardStyle(item: Item, index: number): Record<string, string | number> {
  const zIndex = items.value.length - index;
  if (!item.mounted) {
    return { transform: "translateY(28px) scale(0.9)", opacity: 0, zIndex };
  }
  if (item.removing) {
    const y = expanded.value
      ? -(offsetBefore(index) + index * GAP)
      : -(index * PEEK);
    return { transform: `translateY(${y}px) scale(0.9)`, opacity: 0, zIndex };
  }
  if (expanded.value) {
    const y = -(offsetBefore(index) + index * GAP);
    return { transform: `translateY(${y}px) scale(1)`, opacity: 1, zIndex };
  }
  return {
    transform: `translateY(${-(index * PEEK)}px) scale(${Math.max(0, 1 - index * SCALE_STEP)})`,
    opacity: index < MAX_VISIBLE ? 1 : 0,
    zIndex,
  };
}

function onAdd(msg: unknown): void {
  add(msg as ToastMessage);
}
function onClear(): void {
  items.value.forEach((it) => (it.removing = true));
  timers.forEach((rec) => window.clearTimeout(rec.handle));
  timers.clear();
  window.setTimeout(() => {
    items.value = [];
  }, LEAVE_MS);
}

onMounted(() => {
  ro = new ResizeObserver((entries) => {
    for (const entry of entries) {
      const el = entry.target as HTMLElement;
      const id = Number(el.dataset.toastId);
      if (id) heights[id] = el.offsetHeight;
    }
  });
  ToastEventBus.on("add", onAdd);
  ToastEventBus.on("remove-group", onClear);
  ToastEventBus.on("remove-all-groups", onClear);
});
onBeforeUnmount(() => {
  ToastEventBus.off("add", onAdd);
  ToastEventBus.off("remove-group", onClear);
  ToastEventBus.off("remove-all-groups", onClear);
  timers.forEach((rec) => window.clearTimeout(rec.handle));
  ro?.disconnect();
});
</script>

<template>
  <div
    class="shell-toaster fixed bottom-4 right-4 z-[100] w-89 max-w-[calc(100vw-2rem)]"
    :class="items.length ? 'pointer-events-auto' : 'pointer-events-none'"
    :style="containerStyle"
    @pointerenter="onExpand"
    @pointerleave="onCollapse"
    @focusin="onExpand"
    @focusout="onCollapse"
  >
    <div
      v-for="(item, index) in items"
      :key="item.id"
      :ref="(el) => setCard(item.id, el)"
      :data-toast-id="item.id"
      :role="tone(item.severity).role"
      :aria-live="tone(item.severity).role === 'alert' ? 'assertive' : 'polite'"
      class="shell-toast absolute inset-x-0 bottom-0 flex items-start gap-3 rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-3 text-sm shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5"
      :style="cardStyle(item, index)"
    >
      <AppIcon
        :icon="tone(item.severity).icon"
        :size="18"
        class="mt-0.5 shrink-0"
        :class="tone(item.severity).accent"
      />
      <div class="min-w-0 flex-1">
        <p
          v-if="item.summary"
          class="font-medium text-surface-900 dark:text-surface-0"
        >
          {{ item.summary }}
        </p>
        <p
          v-if="item.detail"
          class="mt-0.5 wrap-break-word text-surface-500 dark:text-surface-400"
        >
          {{ item.detail }}
        </p>
      </div>
      <button
        type="button"
        aria-label="Dismiss"
        class="-mr-1 -mt-1 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-surface-400 outline-none transition-colors hover:bg-surface-100 hover:text-surface-700 focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:hover:bg-surface-800 dark:hover:text-surface-100"
        @click="dismiss(item.id)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
      </button>
    </div>
  </div>
</template>
