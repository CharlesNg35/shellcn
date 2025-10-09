export interface ConnectionTarget {
  id: string
  host: string
  port?: number
  labels?: Record<string, string>
  ordering?: number
}

export interface ConnectionVisibility {
  id?: string
  organization_id?: string | null
  team_id?: string | null
  user_id?: string | null
  permission_scope: string
}

export interface ConnectionMetadata {
  tags?: string[]
  [key: string]: unknown
}

export interface ConnectionSettings {
  host?: string
  port?: number
  [key: string]: unknown
}

export interface ConnectionRecord {
  id: string
  name: string
  description?: string
  protocol_id: string
  organization_id?: string | null
  team_id?: string | null
  owner_user_id?: string | null
  metadata?: ConnectionMetadata
  settings?: ConnectionSettings
  secret_id?: string | null
  last_used_at?: string | null
  targets?: ConnectionTarget[]
  visibility?: ConnectionVisibility[]
}

export type ConnectionStatus = 'ready' | 'connected' | 'disconnected' | 'error' | 'unknown'
