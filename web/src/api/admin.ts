import { api } from "./client";
import type { Role } from "../constants/roles";
import type {
  AdminUser,
  AuditPage,
  UserConnectionSummary,
  UserSummary,
} from "../types/projection";

export interface UserCreate {
  username: string;
  email: string;
  displayName: string;
  role: Role;
  password: string;
}

export interface UserUpdate {
  email: string;
  displayName: string;
  role: Role;
  disabled: boolean;
}

// adminUsersApi centralizes the admin user-management endpoints so route strings
// live in one place.
export const adminUsersApi = {
  list: () => api.get<AdminUser[]>("/admin/users"),
  get: (id: string) => api.get<AdminUser>(`/admin/users/${id}`),
  create: (body: UserCreate) => api.post<AdminUser>("/admin/users", body),
  update: (id: string, body: UserUpdate) =>
    api.put<AdminUser>(`/admin/users/${id}`, body),
  activate: (id: string) => api.post<AdminUser>(`/admin/users/${id}/activate`),
  deactivate: (id: string) =>
    api.post<AdminUser>(`/admin/users/${id}/deactivate`),
  resetTwoFactor: (id: string) =>
    api.post<AdminUser>(`/admin/users/${id}/reset-2fa`),
  connections: (id: string) =>
    api.get<UserConnectionSummary[]>(`/admin/users/${id}/connections`),
  audit: (id: string, limit: number, offset: number) =>
    api.get<AuditPage>(
      `/admin/users/${id}/audit?limit=${limit}&offset=${offset}`,
    ),
  // Directory lookup for the share picker (admin-only enumeration).
  search: (query: string) => {
    const sp = new URLSearchParams();
    if (query.trim()) sp.set("query", query.trim());
    const qs = sp.toString();
    return api.get<UserSummary[]>(`/admin/users/search${qs ? `?${qs}` : ""}`);
  },
};

export const adminSettingsApi = {
  emailStatus: () => api.get<{ enabled: boolean }>("/admin/email"),
};
