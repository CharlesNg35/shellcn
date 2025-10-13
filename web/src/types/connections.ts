export interface ConnectionTarget {
  id: string
  host: string
  port?: number
  labels?: Record<string, string>
  ordering?: number
}

export type ConnectionPrincipalType = 'user' | 'team'

export interface ConnectionSharePrincipal {
  id: string
  type: ConnectionPrincipalType
  name: string
  email?: string
}

export interface ConnectionShare {
  share_id: string
  principal: ConnectionSharePrincipal
  permission_scopes: string[]
  expires_at?: string | null
  granted_by?: ConnectionSharePrincipal | null
  metadata?: Record<string, unknown>
}

export interface ConnectionShareEntry {
  principal: ConnectionSharePrincipal
  granted_by?: ConnectionSharePrincipal | null
  permission_scopes: string[]
  expires_at?: string | null
}

export interface ConnectionShareSummary {
  shared: boolean
  entries: ConnectionShareEntry[]
}

export interface ConnectionMetadata {
  icon?: string
  color?: string
  tags?: string[]
  [key: string]: unknown
}

export interface ConnectionSettings {
  host?: string
  port?: number
  [key: string]: unknown
}

export interface ConnectionFolderSummary {
  id: string
  name: string
  slug?: string
  description?: string
  parent_id?: string | null
  team_id?: string | null
  metadata?: Record<string, unknown>
}

export interface ConnectionFolderNode {
  folder: ConnectionFolderSummary
  connection_count: number
  children?: ConnectionFolderNode[]
}

export interface ConnectionRecord {
  id: string
  name: string
  description?: string
  protocol_id: string
  team_id?: string | null
  owner_user_id?: string | null
  folder_id?: string | null
  metadata?: ConnectionMetadata
  settings?: ConnectionSettings
  identity_id?: string | null
  last_used_at?: string | null
  targets?: ConnectionTarget[]
  shares?: ConnectionShare[]
  share_summary?: ConnectionShareSummary
  folder?: ConnectionFolderSummary
}

export type ConnectionStatus = 'ready' | 'connected' | 'disconnected' | 'error' | 'unknown'

export interface ConnectionProtocolSummary {
  protocol_id: string
  count: number
}

export interface ActiveConnectionSession {
  id: string
  connection_id: string
  connection_name?: string
  user_id: string
  user_name?: string
  team_id?: string | null
  protocol_id: string
  started_at: string
  last_seen_at: string
  host?: string
  port?: number
  metadata?: Record<string, unknown>
}
