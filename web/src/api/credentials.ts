import { api } from "./client";
import type {
  CredentialKindInfo,
  CredentialSummary,
} from "../types/projection";

export interface CredentialFilters {
  kind?: string;
  protocol?: string;
}

export interface CredentialPayload {
  name: string;
  kind: string;
  values: Record<string, string>;
}

function query(f: CredentialFilters): string {
  const sp = new URLSearchParams();
  if (f.kind) sp.set("kind", f.kind);
  if (f.protocol) sp.set("protocol", f.protocol);
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export const credentialsApi = {
  list: (f: CredentialFilters = {}) =>
    api.get<CredentialSummary[]>(`/credentials${query(f)}`),
  create: (body: CredentialPayload) =>
    api.post<CredentialSummary>("/credentials", body),
  update: (id: string, body: CredentialPayload) =>
    api.put<CredentialSummary>(`/credentials/${id}`, body),
  remove: (id: string) => api.del(`/credentials/${id}`),
  kinds: () => api.get<CredentialKindInfo[]>("/credential-kinds"),
};
