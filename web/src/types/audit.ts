import type { ApiMeta } from '@/types/api'

export type AuditLogResult = 'success' | 'failure' | 'denied' | 'error' | 'info' | string

export interface AuditLogUser {
  id: string
  username: string
  email?: string | null
  first_name?: string | null
  last_name?: string | null
}

export interface AuditLogEntry {
  id: string
  user_id?: string | null
  user?: AuditLogUser | null
  username: string
  action: string
  resource?: string | null
  result: AuditLogResult
  ip_address?: string | null
  user_agent?: string | null
  metadata?: unknown
  created_at: string
  updated_at?: string
}

export interface AuditLogListResult {
  data: AuditLogEntry[]
  meta?: ApiMeta
}

export interface AuditLogListParams {
  page?: number
  per_page?: number
  user_id?: string
  actor?: string
  action?: string
  result?: AuditLogResult | 'all'
  resource?: string
  since?: string
  until?: string
  search?: string
}

export type AuditLogExportParams = Omit<AuditLogListParams, 'page' | 'per_page' | 'search'>
