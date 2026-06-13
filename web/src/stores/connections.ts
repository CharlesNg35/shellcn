import { defineStore } from "pinia";
import { ref } from "vue";
import { connectionsApi, connectionFoldersApi } from "../api/connections";
import { pluginsApi } from "../api/plugins";
import type { ConnectionCreate, ConnectionUpdate } from "../api/connections";
import type {
  ConnectionDetail,
  ConnectionFolder,
  ConnectionSummary,
  PluginProjection,
  PluginSummary,
} from "../types/projection";

export const useConnectionsStore = defineStore("connections", () => {
  const connections = ref<ConnectionSummary[]>([]);
  const folders = ref<ConnectionFolder[]>([]);
  const plugins = ref<PluginSummary[]>([]);
  const projections = ref<Record<string, PluginProjection>>({});
  const loaded = ref(false);

  async function load(): Promise<void> {
    const [c, f, p] = await Promise.all([
      connectionsApi.list(),
      connectionFoldersApi.list(),
      pluginsApi.list(),
    ]);
    connections.value = c;
    folders.value = f;
    plugins.value = p;
    loaded.value = true;
  }

  // Projections are fetched on demand and cached — the catalog is never bulk-loaded.
  async function projection(name: string): Promise<PluginProjection> {
    if (!projections.value[name]) {
      const fetched = await pluginsApi.get(name);
      projections.value = { ...projections.value, [name]: fetched };
    }
    return projections.value[name];
  }

  function byId(id: string): ConnectionSummary | undefined {
    return connections.value.find((c) => c.id === id);
  }

  // refresh re-fetches just the connection list after a control-plane mutation.
  async function refresh(): Promise<void> {
    const [c, f] = await Promise.all([
      connectionsApi.list(),
      connectionFoldersApi.list(),
    ]);
    connections.value = c;
    folders.value = f;
  }

  async function refreshPlugins(): Promise<void> {
    plugins.value = await pluginsApi.list();
    projections.value = {};
  }

  async function createFolder(input: {
    name: string;
    color: ConnectionFolder["color"];
    parentId?: string;
  }): Promise<ConnectionFolder> {
    const folder = await connectionFoldersApi.create(input);
    folders.value = [...folders.value, folder].sort(
      (a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name),
    );
    return folder;
  }

  async function updateFolder(
    id: string,
    input: { name: string; color: ConnectionFolder["color"] },
  ): Promise<ConnectionFolder> {
    const folder = await connectionFoldersApi.update(id, input);
    const merged = mergeFolder(
      folder,
      folders.value.find((f) => f.id === id),
    );
    folders.value = folders.value.map((f) => (f.id === id ? merged : f));
    return merged;
  }

  async function createConnection(
    input: ConnectionCreate,
  ): Promise<ConnectionSummary> {
    const created = await connectionsApi.create(input);
    connections.value = [...connections.value, created].sort(
      (a, b) => connectionSortOrder(a) - connectionSortOrder(b),
    );
    return created;
  }

  async function updateConnection(
    id: string,
    input: ConnectionUpdate,
  ): Promise<ConnectionDetail> {
    const updated = await connectionsApi.update(id, input);
    const existing = connections.value.find((c) => c.id === id);
    connections.value = connections.value.map((c) =>
      c.id === id ? mergeConnectionSummary(c, updated, input) : c,
    );
    if (!existing) await refresh();
    return updated;
  }

  async function deleteFolder(id: string): Promise<void> {
    const deleted = folders.value.find((f) => f.id === id);
    const targetParentId = deleted?.parentId;
    await connectionFoldersApi.remove(id);
    folders.value = folders.value.filter((f) => f.id !== id);
    folders.value = folders.value.map((f) =>
      f.parentId === id ? { ...f, parentId: targetParentId } : f,
    );
    connections.value = connections.value.map((c) =>
      c.folderId === id ? { ...c, folderId: targetParentId } : c,
    );
  }

  async function saveLayout(
    items: Array<{
      connectionId: string;
      folderId?: string;
      sortOrder: number;
    }>,
    folderItems: Array<{
      folderId: string;
      parentId?: string;
      sortOrder: number;
    }> = [],
  ): Promise<void> {
    await connectionsApi.saveLayout(items, folderItems);
    const byId = new Map(items.map((i) => [i.connectionId, i]));
    connections.value = connections.value.map((c) => {
      const item = byId.get(c.id);
      return item
        ? {
            ...c,
            folderId: item.folderId || undefined,
            sortOrder: item.sortOrder,
          }
        : c;
    });
    const folderById = new Map(folderItems.map((f) => [f.folderId, f]));
    folders.value = folders.value
      .map((folder) => {
        const item = folderById.get(folder.id);
        return item
          ? { ...folder, parentId: item.parentId, sortOrder: item.sortOrder }
          : folder;
      })
      .sort(
        (a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name),
      );
  }

  return {
    connections,
    folders,
    plugins,
    projections,
    loaded,
    load,
    refresh,
    refreshPlugins,
    createConnection,
    updateConnection,
    createFolder,
    updateFolder,
    deleteFolder,
    saveLayout,
    projection,
    byId,
  };
});

function mergeFolder(
  next: ConnectionFolder,
  existing?: ConnectionFolder,
): ConnectionFolder {
  return {
    ...existing,
    ...next,
    name: next.name,
    color: next.color,
    parentId: next.parentId ?? existing?.parentId,
    sortOrder: next.sortOrder ?? existing?.sortOrder ?? 0,
  };
}

function mergeConnectionSummary(
  existing: ConnectionSummary,
  updated: ConnectionDetail,
  input: ConnectionUpdate,
): ConnectionSummary {
  return {
    ...existing,
    name: updated.name || input.name,
    protocol: updated.protocol || existing.protocol,
    transport: updated.transport || input.transport,
    config: updated.config ?? input.config ?? existing.config,
    recording: updated.recording ?? stringRecord(input.recording),
    aiMode: updated.aiMode ?? input.aiMode,
    aiAllowDestructive:
      updated.aiAllowDestructive ?? input.aiAllowDestructive ?? false,
  };
}

function connectionSortOrder(a: ConnectionSummary): number {
  return a.sortOrder ?? Number.MAX_SAFE_INTEGER;
}

function stringRecord(
  value: Record<string, unknown> | undefined,
): Record<string, string> | undefined {
  if (!value) return undefined;
  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, string] => typeof entry[1] === "string",
    ),
  );
}
