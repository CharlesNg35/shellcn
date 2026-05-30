import { api } from "./client";
import type { GrantAccess, ShareGrant } from "../types/projection";

export type GrantResource = "connections" | "credentials";

export interface GrantRequest {
  access: GrantAccess;
  // Admins grant by subject id (directory pick); operators grant by exact email.
  subjectId?: string;
  email?: string;
}

function base(resource: GrantResource, id: string): string {
  return `/${resource}/${id}/grants`;
}

export const grantsApi = {
  list: (resource: GrantResource, id: string) =>
    api.get<ShareGrant[]>(base(resource, id)),
  create: (resource: GrantResource, id: string, body: GrantRequest) =>
    api.post<ShareGrant>(base(resource, id), body),
  remove: (resource: GrantResource, id: string, grantId: string) =>
    api.del(`${base(resource, id)}/${grantId}`),
};
