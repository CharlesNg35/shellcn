import { defineStore } from "pinia";
import { ref } from "vue";
import type {
  DataSource,
  Icon,
  PanelType,
  Row,
  ResourceRef,
} from "../types/projection";

export interface DockItem {
  id: string;
  title: string;
  icon?: Icon;
  panel: PanelType;
  source: DataSource;
  config?: Record<string, unknown>;
  resource?: ResourceRef | null;
  record?: Row | null;
}

interface DockState {
  items: DockItem[];
  activeId?: string;
  collapsed: boolean;
  height: number;
  dialog?: DockItem | null;
}

const MIN_HEIGHT = 120;
const MAX_HEIGHT = 800;
const DEFAULT_HEIGHT = 280;

// The workspace dock hosts panels (terminals, logs, editors) opened from actions
// as persistent closable tabs, independent of the main view. State is per
// connection so switching connections keeps each dock intact.
export const useDockStore = defineStore("dock", () => {
  const states = ref<Record<string, DockState>>({});

  function state(id: string): DockState {
    if (!states.value[id])
      states.value[id] = {
        items: [],
        collapsed: false,
        height: DEFAULT_HEIGHT,
      };
    return states.value[id];
  }

  function open(id: string, item: DockItem): void {
    const s = state(id);
    if (!s.items.some((i) => i.id === item.id)) s.items.push(item);
    s.activeId = item.id;
    s.collapsed = false;
  }

  function close(id: string, itemId: string): void {
    const s = state(id);
    const idx = s.items.findIndex((i) => i.id === itemId);
    if (idx < 0) return;
    s.items.splice(idx, 1);
    if (s.activeId === itemId) {
      s.activeId = s.items[Math.min(idx, s.items.length - 1)]?.id;
    }
  }

  function activate(id: string, itemId: string): void {
    const s = state(id);
    if (s.items.some((i) => i.id === itemId)) {
      s.activeId = itemId;
      s.collapsed = false;
    }
  }

  function toggleCollapsed(id: string): void {
    const s = state(id);
    s.collapsed = !s.collapsed;
  }

  function setHeight(id: string, height: number): void {
    state(id).height = Math.max(
      MIN_HEIGHT,
      Math.min(MAX_HEIGHT, Math.round(height)),
    );
  }

  function openDialog(id: string, item: DockItem): void {
    state(id).dialog = item;
  }

  function closeDialog(id: string): void {
    state(id).dialog = null;
  }

  return {
    states,
    state,
    open,
    close,
    activate,
    toggleCollapsed,
    setHeight,
    openDialog,
    closeDialog,
  };
});
