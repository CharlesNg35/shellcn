import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "../api/client";
import type {
  ConnectionSummary,
  PluginProjection,
  PluginSummary,
} from "../types/projection";

export const useConnectionsStore = defineStore("connections", () => {
  const connections = ref<ConnectionSummary[]>([]);
  const plugins = ref<PluginSummary[]>([]);
  const projections = ref<Record<string, PluginProjection>>({});
  const loaded = ref(false);

  async function load(): Promise<void> {
    const [c, p] = await Promise.all([
      api.get<ConnectionSummary[]>("/connections"),
      api.get<PluginSummary[]>("/plugins"),
    ]);
    connections.value = c;
    plugins.value = p;
    loaded.value = true;
  }

  // Projections are fetched on demand and cached — the catalog is never bulk-loaded.
  async function projection(name: string): Promise<PluginProjection> {
    if (!projections.value[name]) {
      const fetched = await api.get<PluginProjection>(`/plugins/${name}`);
      projections.value = { ...projections.value, [name]: fetched };
    }
    return projections.value[name];
  }

  function byId(id: string): ConnectionSummary | undefined {
    return connections.value.find((c) => c.id === id);
  }

  // refresh re-fetches just the connection list after a control-plane mutation.
  async function refresh(): Promise<void> {
    connections.value = await api.get<ConnectionSummary[]>("/connections");
  }

  return {
    connections,
    plugins,
    projections,
    loaded,
    load,
    refresh,
    projection,
    byId,
  };
});
