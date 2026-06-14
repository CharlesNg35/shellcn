import type { Icon, Transport } from "./core";
import type { CredentialKind } from "./schema";

export const GrantAccess = {
  Use: "use",
  Manage: "manage",
} as const;
export type GrantAccess = (typeof GrantAccess)[keyof typeof GrantAccess];

export const ConnectionAccess = {
  Owner: "owner",
  Admin: "admin",
  Use: GrantAccess.Use,
  Manage: GrantAccess.Manage,
} as const;
export type ConnectionAccess =
  (typeof ConnectionAccess)[keyof typeof ConnectionAccess];

export const ConnectionStatus = {
  Offline: "offline",
} as const;
export type ConnectionStatus =
  (typeof ConnectionStatus)[keyof typeof ConnectionStatus];

export interface ConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  transport: Transport;
  config?: Record<string, unknown>;
  online?: boolean;
  status?: ConnectionStatus;
  canManage?: boolean;
  canShare?: boolean;
  access?: ConnectionAccess;
  owned?: boolean;
  ownerName?: string;
  sharedWithMe?: boolean;
  sharedByMe?: boolean;
  recording?: Record<string, string>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
  folderId?: string;
  sortOrder?: number;
}

export const FolderColor = {
  Slate: "slate",
  Blue: "blue",
  Teal: "teal",
  Emerald: "emerald",
  Amber: "amber",
  Rose: "rose",
  Violet: "violet",
  Cyan: "cyan",
} as const;
export type FolderColor = (typeof FolderColor)[keyof typeof FolderColor];

export interface ConnectionFolder {
  id: string;
  parentId?: string;
  name: string;
  color: FolderColor;
  sortOrder: number;
}

export interface ShareGrant {
  id: string;
  subjectId: string;
  username?: string;
  displayName?: string;
  access: GrantAccess;
}

export interface CredentialSummary {
  id: string;
  name: string;
  kind: CredentialKind;
  ownerId?: string;
  ownerName?: string;
  values?: Record<string, string>;
  protocols?: string[];
  updatedAt?: string;
}

export interface ConnectionDetail {
  id: string;
  name: string;
  protocol: string;
  transport: Transport;
  ownerId: string;
  config: Record<string, unknown>;
  secrets: Record<string, string>;
  credentials?: Record<string, CredentialRefState>;
  recording?: Record<string, string>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
}

export interface CredentialRefState {
  state: CredentialRefStateKind;
  readable: boolean;
  summary?: CredentialSummary;
}

export const CredentialRefStateKind = {
  Set: "set",
  NotSet: "not_set",
} as const;
export type CredentialRefStateKind =
  (typeof CredentialRefStateKind)[keyof typeof CredentialRefStateKind];

export interface UserConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  createdAt: string;
}
