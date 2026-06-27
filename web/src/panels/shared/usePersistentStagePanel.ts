import {
  nextTick,
  onActivated,
  onDeactivated,
  onMounted,
  onUnmounted,
  ref,
  watch,
  type ComponentPublicInstance,
  type ComputedRef,
  type Ref,
  type WatchSource,
} from "vue";

export interface StageRect {
  top: number;
  left: number;
  width: number;
  height: number;
}

export interface PersistentStagePanelOptions<Handle> {
  stageKey: ComputedRef<string>;
  handle: ComputedRef<Handle | null>;
  watchSource: WatchSource | WatchSource[];
  deep?: boolean;
  register: (handle: Handle) => void;
  activate: (key: string) => void;
  deactivate: (key: string) => void;
  unregister: (key: string) => void;
  updateRect: (key: string, rect: StageRect | null) => void;
}

export interface PersistentStagePanel {
  placeholder: Ref<HTMLElement | null>;
  setPlaceholder: (el: Element | ComponentPublicInstance | null) => void;
  scheduleRectUpdate: () => void;
}

export function usePersistentStagePanel<Handle>(
  options: PersistentStagePanelOptions<Handle>,
): PersistentStagePanel {
  const placeholder = ref<HTMLElement | null>(null);
  const active = ref(true);
  let resizeObserver: ResizeObserver | undefined;
  let observedPlaceholder: HTMLElement | null = null;
  let frame = 0;
  let registeredKey: string | undefined;

  watch(options.watchSource, syncPanel, { deep: options.deep });

  onMounted(() => {
    resizeObserver = new ResizeObserver(scheduleRectUpdate);
    observePlaceholder(placeholder.value);
    window.addEventListener("resize", scheduleRectUpdate);
    window.addEventListener("scroll", scheduleRectUpdate, true);
    syncPanel();
  });

  onActivated(() => {
    active.value = true;
    syncPanel();
  });

  onDeactivated(() => {
    active.value = false;
    if (registeredKey) options.deactivate(registeredKey);
  });

  onUnmounted(() => {
    resizeObserver?.disconnect();
    observedPlaceholder = null;
    window.removeEventListener("resize", scheduleRectUpdate);
    window.removeEventListener("scroll", scheduleRectUpdate, true);
    if (frame) window.cancelAnimationFrame(frame);
    if (registeredKey) options.unregister(registeredKey);
  });

  function syncPanel(): void {
    const handle = options.handle.value;
    if (!handle) {
      if (registeredKey) options.unregister(registeredKey);
      registeredKey = undefined;
      return;
    }
    if (registeredKey && registeredKey !== options.stageKey.value)
      options.unregister(registeredKey);
    registeredKey = options.stageKey.value;
    options.register(handle);
    if (active.value) options.activate(registeredKey);
    void nextTick(scheduleRectUpdate);
  }

  function scheduleRectUpdate(): void {
    if (frame) return;
    frame = window.requestAnimationFrame(() => {
      frame = 0;
      const el = placeholder.value;
      if (!el || !active.value) {
        if (registeredKey) options.updateRect(registeredKey, null);
        return;
      }
      const rect = el.getBoundingClientRect();
      if (!registeredKey) return;
      options.updateRect(registeredKey, {
        top: rect.top,
        left: rect.left,
        width: rect.width,
        height: rect.height,
      });
    });
  }

  function setPlaceholder(el: Element | ComponentPublicInstance | null): void {
    placeholder.value = el instanceof HTMLElement ? el : null;
    observePlaceholder(placeholder.value);
    scheduleRectUpdate();
  }

  function observePlaceholder(el: HTMLElement | null): void {
    if (!resizeObserver) return;
    if (observedPlaceholder === el) return;
    if (observedPlaceholder) resizeObserver.unobserve(observedPlaceholder);
    observedPlaceholder = el;
    if (observedPlaceholder) resizeObserver.observe(observedPlaceholder);
  }

  return { placeholder, setPlaceholder, scheduleRectUpdate };
}
