import { defineStore } from "pinia";
import { ref } from "vue";
import type { ResourceRef, Row } from "../types/projection";

interface ConnectionView {
  activeTab?: string;
  selectedGroup?: string;
  selectedRef?: ResourceRef | null;
  selectedRow?: Row | null;
}

// Per-connection workspace state is kept here (not in components) so that
// remounting a panel or switching connections never loses the active selection.
export const useWorkspaceStore = defineStore("workspace", () => {
  const activeConnectionId = ref<string | null>(null);
  const recent = ref<string[]>([]);
  const views = ref<Record<string, ConnectionView>>({});
  // Connections the user has explicitly connected this session. Drives the
  // sidebar presence dot — protocol-agnostic, unlike a server stream channel
  // (SFTP/Docker/k8s hold a pooled session but no channel). Cleared on reload.
  const connected = ref<Record<string, boolean>>({});

  function view(id: string): ConnectionView {
    if (!views.value[id]) views.value[id] = {};
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

  function selectGroup(id: string, group: string): void {
    const v = view(id);
    v.selectedGroup = group;
    v.selectedRef = null;
    v.selectedRow = null;
  }

  function selectRef(id: string, ref: ResourceRef): void {
    const v = view(id);
    v.selectedRef = ref;
    v.selectedRow = { ref };
  }

  function selectRow(id: string, row: Row): void {
    const v = view(id);
    v.selectedRow = row;
    v.selectedRef = row.ref ?? null;
  }

  function clearSelection(id: string): void {
    const v = view(id);
    v.selectedRef = null;
    v.selectedRow = null;
    v.selectedGroup = undefined;
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
    selectGroup,
    selectRef,
    selectRow,
    clearSelection,
  };
});
