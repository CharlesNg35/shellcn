import { api } from "./client";
import type { Role } from "../constants/roles";

export interface AuthUser {
  id: string;
  username: string;
  displayName?: string;
  email?: string;
  roles: Role[];
  protected?: boolean;
  twoFactorEnabled?: boolean;
}

export interface SessionDTO {
  user: AuthUser;
  csrfToken: string;
  // mfaReminder asks the client to nudge the user to enable 2FA after sign-in.
  mfaReminder: boolean;
}

// LoginResult is either an MFA challenge (second factor pending) or a session.
export interface LoginResult {
  mfaRequired: boolean;
  mfaToken?: string;
  session?: SessionDTO;
}

export interface ProfileUpdate {
  displayName: string;
  email: string;
}

export const authApi = {
  me: () => api.get<SessionDTO>("/auth/me"),
  login: (username: string, password: string) =>
    api.post<LoginResult>("/auth/login", { username, password }),
  loginMfa: (mfaToken: string, code: string) =>
    api.post<LoginResult>("/auth/login/mfa", { mfaToken, code }),
  logout: () => api.post("/auth/logout"),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post<SessionDTO>("/auth/me/password", { currentPassword, newPassword }),
  updateProfile: (body: ProfileUpdate) => api.put<AuthUser>("/auth/me", body),
};
