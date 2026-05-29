import { api } from "./client";
import type {
  AdminUser,
  AuditPage,
  UserConnectionSummary,
} from "../types/projection";

// adminUsersApi centralizes the admin user-management endpoints so route strings
// live in one place.
export const adminUsersApi = {
  list: () => api.get<AdminUser[]>("/admin/users"),
  get: (id: string) => api.get<AdminUser>(`/admin/users/${id}`),
  activate: (id: string) => api.post<AdminUser>(`/admin/users/${id}/activate`),
  deactivate: (id: string) =>
    api.post<AdminUser>(`/admin/users/${id}/deactivate`),
  connections: (id: string) =>
    api.get<UserConnectionSummary[]>(`/admin/users/${id}/connections`),
  audit: (id: string, limit: number, offset: number) =>
    api.get<AuditPage>(
      `/admin/users/${id}/audit?limit=${limit}&offset=${offset}`,
    ),
};
