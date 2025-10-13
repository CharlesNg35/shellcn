import type { ApiMeta, ApiResponse } from '@/types/api'
import { isApiSuccess } from '@/types/api'
import type {
  AuditLogEntry,
  AuditLogExportParams,
  AuditLogListParams,
  AuditLogListResult,
} from '@/types/audit'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const AUDIT_ENDPOINT = '/audit'
const AUDIT_EXPORT_ENDPOINT = '/audit/export'

interface AuditLogUserResponse {
  id: string
  username: string
  email?: string | null
  first_name?: string | null
  last_name?: string | null
}

interface AuditLogResponse {
  id: string
  user_id?: string | null
  user?: AuditLogUserResponse | null
  username: string
  action: string
  resource?: string | null
  result: string
  ip_address?: string | null
  user_agent?: string | null
  metadata?: unknown
  created_at: string
  updated_at?: string
}

function normalizeMetadata(value: unknown): unknown {
  if (value === null || value === undefined) {
    return undefined
  }

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) {
      return undefined
    }
    try {
      return JSON.parse(trimmed)
    } catch {
      return trimmed
    }
  }

  return value
}

function transformAuditLog(raw: AuditLogResponse): AuditLogEntry {
  return {
    id: raw.id,
    user_id: raw.user_id ?? null,
    user: raw.user
      ? {
          id: raw.user.id,
          username: raw.user.username,
          email: raw.user.email ?? undefined,
          first_name: raw.user.first_name ?? undefined,
          last_name: raw.user.last_name ?? undefined,
        }
      : null,
    username: raw.username,
    action: raw.action,
    resource: raw.resource ?? undefined,
    result: raw.result,
    ip_address: raw.ip_address ?? undefined,
    user_agent: raw.user_agent ?? undefined,
    metadata: normalizeMetadata(raw.metadata),
    created_at: raw.created_at,
    updated_at: raw.updated_at,
  }
}

function buildQueryParams(params: AuditLogListParams = {}) {
  const query: Record<string, unknown> = {}

  if (params.page && params.page > 0) {
    query.page = params.page
  }
  if (params.per_page && params.per_page > 0) {
    query.per_page = params.per_page
  }
  if (params.user_id) {
    query.user_id = params.user_id
  }
  if (params.actor) {
    query.actor = params.actor
  }
  if (params.action) {
    query.action = params.action
  }
  if (params.result && params.result !== 'all') {
    query.result = params.result
  }
  if (params.resource) {
    query.resource = params.resource
  }
  if (params.since) {
    query.since = params.since
  }
  if (params.until) {
    query.until = params.until
  }

  return query
}

export async function fetchAuditLogs(params: AuditLogListParams = {}): Promise<AuditLogListResult> {
  const queryParams = buildQueryParams(params)

  const response = await apiClient.get<ApiResponse<AuditLogResponse[]>>(AUDIT_ENDPOINT, {
    params: queryParams,
  })

  const payload = response.data
  const data = unwrapResponse(response)
  const meta: ApiMeta | undefined = isApiSuccess(payload) ? payload.meta : undefined

  return {
    data: data.map(transformAuditLog),
    meta,
  }
}

export async function exportAuditLogs(params: AuditLogExportParams = {}): Promise<AuditLogEntry[]> {
  const queryParams = buildQueryParams(params)

  const response = await apiClient.get<ApiResponse<AuditLogResponse[]>>(AUDIT_EXPORT_ENDPOINT, {
    params: queryParams,
  })

  const data = unwrapResponse(response)
  return data.map(transformAuditLog)
}

export const auditApi = {
  list: fetchAuditLogs,
  export: exportAuditLogs,
}
