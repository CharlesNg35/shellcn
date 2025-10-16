import type { ApiMeta, ApiResponse } from '@/types/api'
import type {
  ActiveConnectionSession,
  ConnectionRecord,
  ConnectionTarget,
  ConnectionShare,
  ConnectionShareEntry,
  ConnectionShareSummary,
  ConnectionSharePrincipal,
  ConnectionProtocolSummary,
  ConnectionSettings,
} from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import { isApiSuccess } from '@/types/api'

const CONNECTIONS_ENDPOINT = '/connections'
const ACTIVE_CONNECTIONS_ENDPOINT = '/connections/active'

export interface ConnectionCreatePayload {
  name: string
  description?: string
  protocol_id: string
  team_id?: string | null
  folder_id?: string | null
  metadata?: Record<string, unknown>
  settings?: Record<string, unknown>
  grant_team_permissions?: string[]
  identity_id?: string | null
}

interface ConnectionTargetResponse {
  id: string
  host: string
  port?: number
  labels?: Record<string, unknown>
  ordering?: number
}

interface ConnectionShareActorResponse {
  id?: string
  type?: string
  name?: string
  email?: string | null
}

interface ConnectionShareResponse {
  share_id?: string
  principal?: ConnectionShareActorResponse
  permission_scopes?: unknown
  expires_at?: string | null
  granted_by?: ConnectionShareActorResponse | null
  metadata?: Record<string, unknown> | string | null
}

interface ConnectionShareEntryResponse {
  principal?: ConnectionShareActorResponse
  granted_by?: ConnectionShareActorResponse | null
  permission_scopes?: unknown
  expires_at?: string | null
}

interface ConnectionShareSummaryResponse {
  shared?: boolean
  entries?: ConnectionShareEntryResponse[] | null
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
  identity_id?: string | null
  last_used_at?: string | null
  targets?: ConnectionTargetResponse[]
  shares?: ConnectionShareResponse[] | null
  share_summary?: ConnectionShareSummaryResponse | null
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

function normaliseConnectionSettings(
  value?: Record<string, unknown> | string | null
): ConnectionSettings | undefined {
  const settings = coerceObject<ConnectionSettings>(value)
  if (!settings) {
    return undefined
  }
  const normalised: ConnectionSettings = { ...settings }
  if (Object.prototype.hasOwnProperty.call(settings, 'recording_enabled')) {
    normalised.recording_enabled = Boolean(settings.recording_enabled)
  }
  return normalised
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

function normaliseStringArray(value: unknown): string[] {
  if (!value) {
    return []
  }
  if (Array.isArray(value)) {
    return value
      .map((scope) => (typeof scope === 'string' ? scope.trim() : String(scope).trim()))
      .filter((scope) => scope.length > 0)
  }
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value)
      if (Array.isArray(parsed)) {
        return parsed
          .map((scope) => (typeof scope === 'string' ? scope.trim() : String(scope).trim()))
          .filter((scope) => scope.length > 0)
      }
    } catch {
      return value
        .split(',')
        .map((scope) => scope.trim())
        .filter((scope) => scope.length > 0)
    }
  }
  return []
}

function transformShareActor(
  raw?: ConnectionShareActorResponse | null
): ConnectionSharePrincipal | undefined {
  if (!raw?.id) {
    return undefined
  }
  const type = typeof raw.type === 'string' && raw.type.trim().length > 0 ? raw.type.trim() : 'user'
  return {
    id: raw.id,
    type: type as ConnectionSharePrincipal['type'],
    name: raw.name ?? raw.id,
    email: raw.email ?? undefined,
  }
}

function transformShares(shares?: ConnectionShareResponse[] | null): ConnectionShare[] {
  if (!shares?.length) {
    return []
  }

  const result: ConnectionShare[] = []
  shares.forEach((share) => {
    const principal = transformShareActor(share.principal)
    if (!principal) {
      return
    }

    const entry: ConnectionShare = {
      share_id: share.share_id ?? `${principal.type}:${principal.id}`,
      principal,
      permission_scopes: normaliseStringArray(share.permission_scopes),
    }

    const grantedBy = transformShareActor(share.granted_by)
    if (grantedBy !== undefined) {
      entry.granted_by = grantedBy ?? null
    }
    if (share.expires_at !== undefined) {
      entry.expires_at = share.expires_at
    }
    if (share.metadata && typeof share.metadata === 'object') {
      entry.metadata = share.metadata as Record<string, unknown>
    }

    result.push(entry)
  })

  return result
}

function transformShareSummary(
  raw?: ConnectionShareSummaryResponse | null
): ConnectionShareSummary | undefined {
  if (!raw) {
    return undefined
  }

  const entries: ConnectionShareEntry[] = []
  ;(raw.entries ?? []).forEach((entry) => {
    const principal = transformShareActor(entry.principal)
    if (!principal) {
      return
    }

    const transformed: ConnectionShareEntry = {
      principal,
      permission_scopes: normaliseStringArray(entry.permission_scopes),
    }

    const grantedBy = transformShareActor(entry.granted_by)
    if (grantedBy !== undefined) {
      transformed.granted_by = grantedBy ?? null
    }
    if (entry.expires_at !== undefined) {
      transformed.expires_at = entry.expires_at
    }

    entries.push(transformed)
  })

  return {
    shared: raw.shared ?? entries.length > 0,
    entries,
  }
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
    settings: normaliseConnectionSettings(raw.settings),
    identity_id: raw.identity_id ?? null,
    last_used_at: raw.last_used_at ?? null,
    targets: transformTargets(raw.targets),
    shares: transformShares(raw.shares),
    share_summary: transformShareSummary(raw.share_summary),
    folder: raw.folder
      ? {
          id: raw.folder.id,
          name: raw.folder.name,
          slug: raw.folder.slug,
          description: raw.folder.description,
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

export interface ConnectionSharePayload {
  user_id?: string | null
  team_id?: string | null
  permission_scopes: string[]
  expires_at?: string | null
  metadata?: Record<string, unknown>
}

export interface FetchActiveConnectionSessionsParams {
  protocol_id?: string
  team_id?: string
  scope?: 'personal' | 'team' | 'all'
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

export async function fetchActiveConnectionSessions(
  params?: FetchActiveConnectionSessionsParams
): Promise<ActiveConnectionSession[]> {
  const response = await apiClient.get<ApiResponse<ActiveConnectionSession[]>>(
    ACTIVE_CONNECTIONS_ENDPOINT,
    {
      params,
    }
  )
  const data = unwrapResponse(response)
  return Array.isArray(data) ? data : []
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

export async function createConnection(
  payload: ConnectionCreatePayload
): Promise<ConnectionRecord> {
  const response = await apiClient.post<ApiResponse<ConnectionResponse>>(
    CONNECTIONS_ENDPOINT,
    payload
  )
  const data = unwrapResponse(response)
  return transformConnection(data)
}

export async function fetchConnectionShares(connectionId: string): Promise<ConnectionShare[]> {
  const response = await apiClient.get<ApiResponse<ConnectionShareResponse[]>>(
    `${CONNECTIONS_ENDPOINT}/${connectionId}/shares`
  )
  const data = unwrapResponse(response)
  return transformShares(data)
}

export async function createConnectionShare(
  connectionId: string,
  payload: ConnectionSharePayload
): Promise<ConnectionShare> {
  const response = await apiClient.post<ApiResponse<ConnectionShareResponse>>(
    `${CONNECTIONS_ENDPOINT}/${connectionId}/shares`,
    payload
  )
  const data = unwrapResponse(response)
  const [share] = transformShares([data])
  return share
}

export async function deleteConnectionShare(
  connectionId: string,
  shareId: string
): Promise<boolean> {
  const response = await apiClient.delete<ApiResponse<{ deleted: boolean }>>(
    `${CONNECTIONS_ENDPOINT}/${connectionId}/shares/${shareId}`
  )
  const data = unwrapResponse(response)
  return Boolean((data as { deleted?: boolean })?.deleted)
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
