import { api } from "./client";
import type {
  ConnectionDetail,
  ConnectionFolder,
  ConnectionSummary,
} from "../types/projection";

export interface ConnectionCreate {
  name: string;
  protocol: string;
  transport: string;
  config: Record<string, unknown>;
  preserveCredentials: string[];
  recording: Record<string, unknown>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
}

export interface ConnectionUpdate {
  name: string;
  transport: string;
  config: Record<string, unknown>;
  preserveCredentials: string[];
  recording: Record<string, unknown>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
}

export interface LayoutItem {
  connectionId: string;
  folderId?: string;
  sortOrder: number;
}

export interface LayoutFolderItem {
  folderId: string;
  parentId?: string;
  sortOrder: number;
}

export const connectionsApi = {
  list: () => api.get<ConnectionSummary[]>("/connections"),
  get: (id: string) => api.get<ConnectionDetail>(`/connections/${id}`),
  create: (body: ConnectionCreate) =>
    api.post<ConnectionSummary>("/connections", body),
  update: (id: string, body: ConnectionUpdate) =>
    api.put<ConnectionDetail>(`/connections/${id}`, body),
  remove: (id: string) => api.del(`/connections/${id}`),
  saveLayout: (items: LayoutItem[], folders: LayoutFolderItem[]) =>
    api.put("/connections/layout", { items, folders }),
};

export interface FolderCreate {
  name: string;
  color: ConnectionFolder["color"];
  parentId?: string;
}

export interface FolderUpdate {
  name: string;
  color: ConnectionFolder["color"];
}

export const connectionFoldersApi = {
  list: () => api.get<ConnectionFolder[]>("/connection-folders"),
  create: (body: FolderCreate) =>
    api.post<ConnectionFolder>("/connection-folders", body),
  update: (id: string, body: FolderUpdate) =>
    api.put<ConnectionFolder>(`/connection-folders/${id}`, body),
  remove: (id: string) => api.del(`/connection-folders/${id}`),
};
