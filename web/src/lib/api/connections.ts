import type { ApiMeta, ApiResponse } from '@/types/api'
import type {
  ConnectionRecord,
  ConnectionTarget,
  ConnectionVisibility,
  ConnectionProtocolSummary,
} from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import { isApiSuccess } from '@/types/api'

const CONNECTIONS_ENDPOINT = '/connections'

export interface ConnectionCreatePayload {
  name: string
  description?: string
  protocol_id: string
  team_id?: string | null
  folder_id?: string | null
  metadata?: Record<string, unknown>
  settings?: Record<string, unknown>
}

interface ConnectionTargetResponse {
  id: string
  host: string
  port?: number
  labels?: Record<string, unknown>
  ordering?: number
}

interface ConnectionVisibilityResponse {
  id?: string
  team_id?: string | null
  user_id?: string | null
  permission_scope: string
}

interface ConnectionFolderResponse {
  id: string
  name: string
  slug?: string
  description?: string
  icon?: string
  color?: string
  parent_id?: string | null
  team_id?: string | null
  metadata?: Record<string, unknown> | string | null
}

interface ConnectionResponse {
  id: string
  name: string
  description?: string
  protocol_id: string
  team_id?: string | null
  owner_user_id?: string | null
  folder_id?: string | null
  metadata?: Record<string, unknown> | string | null
  settings?: Record<string, unknown> | string | null
  secret_id?: string | null
  last_used_at?: string | null
  targets?: ConnectionTargetResponse[]
  visibility?: ConnectionVisibilityResponse[]
  folder?: ConnectionFolderResponse | null
}

function coerceObject<T extends Record<string, unknown>>(
  value?: Record<string, unknown> | string | null
): T | undefined {
  if (!value) {
    return undefined
  }
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value) as Record<string, unknown>
      return parsed as T
    } catch {
      return undefined
    }
  }
  return value as T
}

function transformTargets(targets?: ConnectionTargetResponse[]): ConnectionTarget[] {
  if (!targets?.length) {
    return []
  }
  return targets.map((target) => ({
    id: target.id,
    host: target.host,
    port: target.port,
    labels: target.labels
      ? Object.entries(target.labels).reduce<Record<string, string>>((acc, [key, value]) => {
          if (typeof value === 'string') {
            acc[key] = value
          } else if (value != null) {
            acc[key] = JSON.stringify(value)
          }
          return acc
        }, {})
      : undefined,
    ordering: target.ordering,
  }))
}

function transformVisibility(visibility?: ConnectionVisibilityResponse[]): ConnectionVisibility[] {
  if (!visibility?.length) {
    return []
  }
  return visibility.map((entry) => ({
    id: entry.id,
    team_id: entry.team_id ?? null,
    user_id: entry.user_id ?? null,
    permission_scope: entry.permission_scope,
  }))
}

function transformConnection(raw: ConnectionResponse): ConnectionRecord {
  return {
    id: raw.id,
    name: raw.name,
    description: raw.description,
    protocol_id: raw.protocol_id,
    team_id: raw.team_id ?? null,
    owner_user_id: raw.owner_user_id ?? null,
    folder_id: raw.folder_id ?? null,
    metadata: coerceObject(raw.metadata),
    settings: coerceObject(raw.settings),
    secret_id: raw.secret_id ?? null,
    last_used_at: raw.last_used_at ?? null,
    targets: transformTargets(raw.targets),
    visibility: transformVisibility(raw.visibility),
    folder: raw.folder
      ? {
          id: raw.folder.id,
          name: raw.folder.name,
          slug: raw.folder.slug,
          description: raw.folder.description,
          icon: raw.folder.icon,
          color: raw.folder.color,
          parent_id: raw.folder.parent_id ?? null,
          team_id: raw.folder.team_id ?? null,
          metadata: coerceObject(raw.folder.metadata),
        }
      : undefined,
  }
}

export interface FetchConnectionsParams {
  protocol_id?: string
  team_id?: string
  folder_id?: string
  search?: string
  include?: string
  page?: number
  per_page?: number
}

export interface ConnectionListResult {
  data: ConnectionRecord[]
  meta?: ApiMeta
}

export async function fetchConnections(
  params?: FetchConnectionsParams
): Promise<ConnectionListResult> {
  const response = await apiClient.get<ApiResponse<ConnectionResponse[]>>(CONNECTIONS_ENDPOINT, {
    params,
  })
  const payload = response.data
  const data = unwrapResponse(response)
  const meta = isApiSuccess(payload) ? payload.meta : undefined
  return {
    data: data.map(transformConnection),
    meta,
  }
}

export async function fetchConnectionById(id: string, include?: string): Promise<ConnectionRecord> {
  const response = await apiClient.get<ApiResponse<ConnectionResponse>>(
    `${CONNECTIONS_ENDPOINT}/${id}`,
    {
      params: include ? { include } : undefined,
    }
  )
  const data = unwrapResponse(response)
  return transformConnection(data)
}

export async function createConnection(payload: ConnectionCreatePayload): Promise<ConnectionRecord> {
  const response = await apiClient.post<ApiResponse<ConnectionResponse>>(CONNECTIONS_ENDPOINT, payload)
  const data = unwrapResponse(response)
  return transformConnection(data)
}

interface ConnectionSummaryResponse {
  protocol_id: string
  count: number
}

export async function fetchConnectionSummary(params?: {
  team_id?: string
}): Promise<ConnectionProtocolSummary[]> {
  const response = await apiClient.get<ApiResponse<ConnectionSummaryResponse[]>>(
    `${CONNECTIONS_ENDPOINT}/summary`,
    {
      params,
    }
  )
  const data = unwrapResponse(response)
  return data.map((item) => ({ protocol_id: item.protocol_id, count: item.count }))
}
