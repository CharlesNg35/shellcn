import { api } from "./client";
import type { PluginProjection, PluginSummary } from "../types/projection";

export const pluginsApi = {
  list: () => api.get<PluginSummary[]>("/plugins"),
  get: (name: string) => api.get<PluginProjection>(`/plugins/${name}`),
};
