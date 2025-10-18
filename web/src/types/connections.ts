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

export interface ConnectionTemplateMetadata {
  driver_id?: string
  version?: string
  fields?: Record<string, unknown>
  [key: string]: unknown
}

export interface ConnectionMetadata {
  icon?: string
  color?: string
  tags?: string[]
  connection_template?: ConnectionTemplateMetadata
  [key: string]: unknown
}

export interface ConnectionSettings {
  host?: string
  port?: number
  recording_enabled?: boolean
  concurrent_limit?: number
  idle_timeout_minutes?: number
  enable_sftp?: boolean
  terminal_config_override?: ConnectionTerminalConfigOverride
  [key: string]: unknown
}

export interface ConnectionTerminalConfigOverride {
  font_family?: string
  font_size?: number
  scrollback_limit?: number
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

export interface ActiveSessionParticipant {
  session_id: string
  user_id: string
  user_name?: string
  role: string
  access_mode: string
  joined_at: string
  is_owner?: boolean
  is_write_holder?: boolean
}

export interface ActiveSessionCapabilities {
  panes?: string[]
  features?: Record<string, boolean>
  [key: string]: unknown
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
  descriptor_id?: string
  capabilities?: ActiveSessionCapabilities
  template?: ConnectionTemplateMetadata
  concurrent_limit?: number
  owner_user_id?: string
  owner_user_name?: string
  write_holder?: string
  participants?: Record<string, ActiveSessionParticipant>
}

export interface SessionParticipantsSummary {
  session_id: string
  connection_id: string
  owner_user_id: string
  owner_user_name?: string
  write_holder?: string | null
  participants: ActiveSessionParticipant[]
}
