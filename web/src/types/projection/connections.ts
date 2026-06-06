import type { Icon, Transport } from "./core";
import type { CredentialKind } from "./schema";

export type GrantAccess = "use" | "manage";

export interface ConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  transport: Transport;
  online?: boolean;
  status?: "offline";
  canManage?: boolean;
  canShare?: boolean;
  access?: "owner" | "admin" | GrantAccess;
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

export type FolderColor =
  | "slate"
  | "blue"
  | "teal"
  | "emerald"
  | "amber"
  | "rose"
  | "violet"
  | "cyan";

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
  identity?: string;
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
  state: "set" | "not_set";
  readable: boolean;
  summary?: CredentialSummary;
}

export interface UserConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  createdAt: string;
}
