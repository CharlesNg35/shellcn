import type { ApiResponse } from '@/types/api'
import type { ConnectionRecord, ConnectionTarget, ConnectionVisibility } from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const CONNECTIONS_ENDPOINT = '/connections'

interface ConnectionTargetResponse {
  id: string
  host: string
  port?: number
  labels?: Record<string, string>
  ordering?: number
}

interface ConnectionVisibilityResponse {
  id?: string
  organization_id?: string | null
  team_id?: string | null
  user_id?: string | null
  permission_scope: string
}

interface ConnectionResponse {
  id: string
  name: string
  description?: string
  protocol_id: string
  organization_id?: string | null
  team_id?: string | null
  owner_user_id?: string | null
  metadata?: Record<string, unknown> | string | null
  settings?: Record<string, unknown> | string | null
  secret_id?: string | null
  last_used_at?: string | null
  targets?: ConnectionTargetResponse[]
  visibility?: ConnectionVisibilityResponse[]
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
    labels: target.labels,
    ordering: target.ordering,
  }))
}

function transformVisibility(visibility?: ConnectionVisibilityResponse[]): ConnectionVisibility[] {
  if (!visibility?.length) {
    return []
  }
  return visibility.map((entry) => ({
    id: entry.id,
    organization_id: entry.organization_id ?? null,
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
    organization_id: raw.organization_id ?? null,
    team_id: raw.team_id ?? null,
    owner_user_id: raw.owner_user_id ?? null,
    metadata: coerceObject(raw.metadata),
    settings: coerceObject(raw.settings),
    secret_id: raw.secret_id ?? null,
    last_used_at: raw.last_used_at ?? null,
    targets: transformTargets(raw.targets),
    visibility: transformVisibility(raw.visibility),
  }
}

export async function fetchConnections(): Promise<ConnectionRecord[]> {
  const response = await apiClient.get<ApiResponse<ConnectionResponse[]>>(CONNECTIONS_ENDPOINT)
  const data = unwrapResponse(response)
  return data.map(transformConnection)
}
