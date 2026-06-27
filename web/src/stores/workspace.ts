import { defineStore } from "pinia";
import { ref } from "vue";
import { KEEP_ALIVE_WORKBENCH_TABS_MAX } from "./sessionLimits";
import type { Icon, ResourceIdentity, Row } from "../types/projection";

export interface OpenView {
  id: string;
  title: string;
  subtitle?: string;
  icon?: Icon;
  kind: "detail" | "list";
  ref?: ResourceIdentity;
  row?: Row;
  resourceKind?: string;
  groupKey?: string;
  params?: Record<string, string>;
  preview?: boolean;
}

interface ConnectionView {
  activeTab?: string;
  views: OpenView[];
  activeViewId?: string;
}

interface ConnectionLayout {
  treeSidebarWidth: number;
}

export interface TableViewState {
  filterText: string;
  sortField?: string;
  sortOrder?: number;
  first: number;
  pageSize: number;
}

export const DEFAULT_TREE_SIDEBAR_WIDTH = 256;
export const MIN_TREE_SIDEBAR_WIDTH = 192;
export const MAX_TREE_SIDEBAR_WIDTH = 520;

export const useWorkspaceStore = defineStore("workspace", () => {
  const activeConnectionId = ref<string | null>(null);
  const recent = ref<string[]>([]);
  const views = ref<Record<string, ConnectionView>>({});
  const layouts = ref<Record<string, ConnectionLayout>>({});
  const tableStates = ref<Record<string, TableViewState>>({});
  const connected = ref<Record<string, boolean>>({});
  const connectedOrder = ref<string[]>([]);

  function view(id: string): ConnectionView {
    if (!views.value[id]) views.value[id] = { views: [] };
    return views.value[id];
  }

  function layout(id: string): ConnectionLayout {
    if (!layouts.value[id]) {
      layouts.value[id] = {
        treeSidebarWidth: DEFAULT_TREE_SIDEBAR_WIDTH,
      };
    }
    return layouts.value[id];
  }

  function setTreeSidebarWidth(id: string, width: number): void {
    layout(id).treeSidebarWidth = Math.min(
      MAX_TREE_SIDEBAR_WIDTH,
      Math.max(MIN_TREE_SIDEBAR_WIDTH, Math.round(width)),
    );
  }

  function setConnected(id: string, on: boolean): void {
    if (on) {
      connected.value[id] = true;
      connectedOrder.value = [
        ...connectedOrder.value.filter((candidate) => candidate !== id),
        id,
      ];
      return;
    }
    delete connected.value[id];
    connectedOrder.value = connectedOrder.value.filter(
      (candidate) => candidate !== id,
    );
  }

  function isConnected(id: string): boolean {
    return Boolean(connected.value[id]);
  }

  function connectedIds(): string[] {
    const live = new Set(Object.keys(connected.value));
    connectedOrder.value = connectedOrder.value.filter((id) => live.has(id));
    for (const id of live) {
      if (!connectedOrder.value.includes(id)) connectedOrder.value.push(id);
    }
    return [...connectedOrder.value];
  }

  function open(id: string): void {
    activeConnectionId.value = id;
    recent.value = [id, ...recent.value.filter((r) => r !== id)].slice(0, 10);
    view(id);
  }

  function setActiveTab(id: string, tab: string): void {
    view(id).activeTab = tab;
  }

  function openView(id: string, v: OpenView): void {
    const c = view(id);
    if (!c.views.some((x) => x.id === v.id))
      c.views.push({ ...v, preview: false });
    c.activeViewId = v.id;
    while (c.views.length > KEEP_ALIVE_WORKBENCH_TABS_MAX) {
      const idx = c.views.findIndex((x) => x.id !== c.activeViewId);
      if (idx < 0) break;
      c.views.splice(idx, 1);
    }
  }

  function openPreviewView(id: string, v: OpenView): void {
    const c = view(id);
    if (c.views.some((x) => x.id === v.id)) {
      c.activeViewId = v.id;
      return;
    }
    const previewIdx = c.views.findIndex((x) => x.preview);
    const preview = { ...v, preview: true };
    if (previewIdx >= 0) c.views.splice(previewIdx, 1, preview);
    else c.views.push(preview);
    c.activeViewId = v.id;
    while (c.views.length > KEEP_ALIVE_WORKBENCH_TABS_MAX) {
      const idx = c.views.findIndex((x) => x.id !== c.activeViewId);
      if (idx < 0) break;
      c.views.splice(idx, 1);
    }
  }

  function pinView(id: string, viewId: string): void {
    const tab = view(id).views.find((v) => v.id === viewId);
    if (tab) tab.preview = false;
  }

  function closeView(id: string, viewId: string): void {
    const c = view(id);
    const idx = c.views.findIndex((v) => v.id === viewId);
    if (idx < 0) return;
    c.views.splice(idx, 1);
    if (c.activeViewId === viewId) {
      c.activeViewId = c.views[Math.min(idx, c.views.length - 1)]?.id;
    }
  }

  function activateView(id: string, viewId: string): void {
    const c = view(id);
    if (c.views.some((v) => v.id === viewId)) c.activeViewId = viewId;
  }

  function setViews(id: string, next: OpenView[]): void {
    view(id).views = next;
  }

  function activeView(id: string): OpenView | undefined {
    const c = view(id);
    return c.views.find((v) => v.id === c.activeViewId);
  }

  function clearViews(id: string): void {
    const c = view(id);
    c.views = [];
    c.activeViewId = undefined;
  }

  function tableState(key: string, defaults: TableViewState): TableViewState {
    if (!tableStates.value[key]) tableStates.value[key] = { ...defaults };
    return tableStates.value[key];
  }

  function setTableState(key: string, state: TableViewState): void {
    tableStates.value[key] = { ...state };
  }

  return {
    activeConnectionId,
    recent,
    views,
    layouts,
    tableStates,
    connected,
    connectedOrder,
    view,
    layout,
    setTreeSidebarWidth,
    open,
    setConnected,
    isConnected,
    connectedIds,
    setActiveTab,
    openView,
    openPreviewView,
    pinView,
    closeView,
    activateView,
    setViews,
    activeView,
    clearViews,
    tableState,
    setTableState,
  };
});
