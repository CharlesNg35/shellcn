import type { Role } from "../../constants/roles";

export interface UserSummary {
  id: string;
  username: string;
  displayName?: string;
}

export interface AdminUser {
  id: string;
  username: string;
  email?: string;
  displayName?: string;
  roles: Role[];
  disabled: boolean;
  protected: boolean;
  twoFactorEnabled?: boolean;
}

export interface AuditEntry {
  id: string;
  time: string;
  event: string;
  risk?: string;
  result: string;
  connectionId?: string;
  error?: string;
  remoteAddr?: string;
}

export interface AuditPage {
  items: AuditEntry[];
  total: number;
}

export interface InvitationSummary {
  id: string;
  email: string;
  role: string;
  status: string;
  createdAt: string;
  expiresAt: string;
}

export interface InviteResult {
  invitation: InvitationSummary;
  link: string;
  emailSent: boolean;
}
