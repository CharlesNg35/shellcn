import { api } from "./client";
import type { Role } from "../constants/roles";

export interface AuthUser {
  id: string;
  username: string;
  displayName?: string;
  email?: string;
  roles: Role[];
  protected?: boolean;
}

export interface SessionDTO {
  user: AuthUser;
  csrfToken: string;
}

export interface ProfileUpdate {
  displayName: string;
  email: string;
}

export const authApi = {
  me: () => api.get<SessionDTO>("/auth/me"),
  login: (username: string, password: string) =>
    api.post<SessionDTO>("/auth/login", { username, password }),
  logout: () => api.post("/auth/logout"),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post<SessionDTO>("/auth/me/password", { currentPassword, newPassword }),
  updateProfile: (body: ProfileUpdate) => api.put<AuthUser>("/auth/me", body),
};
