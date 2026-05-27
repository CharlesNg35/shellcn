import { defineStore } from "pinia";
import { ref } from "vue";
import type { Icon, ResourceRef, Row } from "../types/projection";

// An open view in the sidebar-tree workspace: either a resource detail or a
// resource-kind list. Multiple stay open as a closable tab strip.
export interface OpenView {
  id: string;
  title: string;
  icon?: Icon;
  kind: "detail" | "list";
  // detail
  ref?: ResourceRef;
  row?: Row;
  // list
  resourceKind?: string;
  groupKey?: string;
  params?: Record<string, string>;
}

interface ConnectionView {
  activeTab?: string;
  views: OpenView[];
  activeViewId?: string;
}

// Per-connection workspace state is kept here (not in components) so that
// remounting a panel or switching connections never loses open views.
export const useWorkspaceStore = defineStore("workspace", () => {
  const activeConnectionId = ref<string | null>(null);
  const recent = ref<string[]>([]);
  const views = ref<Record<string, ConnectionView>>({});
  // Connections the user has explicitly connected this session. Drives the
  // sidebar presence dot without assuming a live stream channel. Cleared on reload.
  const connected = ref<Record<string, boolean>>({});

  function view(id: string): ConnectionView {
    if (!views.value[id]) views.value[id] = { views: [] };
    return views.value[id];
  }

  function setConnected(id: string, on: boolean): void {
    if (on) connected.value[id] = true;
    else delete connected.value[id];
  }

  function isConnected(id: string): boolean {
    return Boolean(connected.value[id]);
  }

  function open(id: string): void {
    activeConnectionId.value = id;
    recent.value = [id, ...recent.value.filter((r) => r !== id)].slice(0, 10);
    view(id);
  }

  function setActiveTab(id: string, tab: string): void {
    view(id).activeTab = tab;
  }

  // openView adds a view (or re-activates an already-open one) and makes it
  // active — the basis of the multi-open workbench tab strip.
  function openView(id: string, v: OpenView): void {
    const c = view(id);
    if (!c.views.some((x) => x.id === v.id)) c.views.push(v);
    c.activeViewId = v.id;
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

  function activeView(id: string): OpenView | undefined {
    const c = view(id);
    return c.views.find((v) => v.id === c.activeViewId);
  }

  function clearViews(id: string): void {
    const c = view(id);
    c.views = [];
    c.activeViewId = undefined;
  }

  return {
    activeConnectionId,
    recent,
    views,
    connected,
    view,
    open,
    setConnected,
    isConnected,
    setActiveTab,
    openView,
    closeView,
    activateView,
    activeView,
    clearViews,
  };
});
