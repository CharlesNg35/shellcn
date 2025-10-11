import type { ApiResponse } from '@/types/api'
import type {
  Protocol,
  ProtocolCapabilities,
  ProtocolListResult,
  ProtocolPermission,
} from '@/types/protocols'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const PROTOCOLS_ENDPOINT = '/protocols'
const PROTOCOLS_AVAILABLE_ENDPOINT = '/protocols/available'
const PROTOCOL_PERMISSIONS_ENDPOINT = (protocolId: string) => `/protocols/${protocolId}/permissions`

interface ProtocolResponse {
  id: string
  name: string
  module: string
  description?: string
  category?: string
  icon?: string
  default_port?: number
  sort_order?: number
  features?: string[] | null
  capabilities?: ProtocolCapabilitiesResponse | null
  driver_enabled: boolean
  config_enabled: boolean
  available: boolean
  permissions?: ProtocolPermissionResponse[] | null
}

interface ProtocolCapabilitiesResponse {
  terminal?: boolean
  desktop?: boolean
  file_transfer?: boolean
  clipboard?: boolean
  session_recording?: boolean
  metrics?: boolean
  reconnect?: boolean
  extras?: Record<string, boolean>
}

interface ProtocolPermissionResponse {
  id?: string
  display_name?: string
  description?: string
  category?: string
  default_scope?: string
  module?: string
  depends_on?: unknown
  implies?: unknown
  metadata?: Record<string, unknown> | string | null
}

function normalizeCapabilities(input?: ProtocolCapabilitiesResponse | null): ProtocolCapabilities {
  return {
    terminal: Boolean(input?.terminal),
    desktop: Boolean(input?.desktop),
    file_transfer: Boolean(input?.file_transfer),
    clipboard: Boolean(input?.clipboard),
    session_recording: Boolean(input?.session_recording),
    metrics: Boolean(input?.metrics),
    reconnect: Boolean(input?.reconnect),
    extras: input?.extras ?? {},
  }
}

function normalizeStringArrayPayload(source: unknown): string[] {
  if (!source) {
    return []
  }
  if (Array.isArray(source)) {
    return source
      .map((value) => (typeof value === 'string' ? value.trim() : String(value).trim()))
      .filter((value) => value.length > 0)
  }
  if (typeof source === 'string' && source.trim().length > 0) {
    try {
      const parsed = JSON.parse(source)
      if (Array.isArray(parsed)) {
        return parsed
          .map((value) => (typeof value === 'string' ? value.trim() : String(value).trim()))
          .filter((value) => value.length > 0)
      }
    } catch {
      return source
        .split(',')
        .map((value) => value.trim())
        .filter((value) => value.length > 0)
    }
  }
  return []
}

function transformProtocolPermission(raw: ProtocolPermissionResponse): ProtocolPermission | null {
  if (!raw) {
    return null
  }
  const id = typeof raw.id === 'string' && raw.id.trim().length > 0 ? raw.id.trim() : ''
  if (!id) {
    return null
  }

  let metadata: Record<string, unknown> | undefined
  if (raw.metadata && typeof raw.metadata === 'object') {
    metadata = raw.metadata
  } else if (typeof raw.metadata === 'string' && raw.metadata.trim().length > 0) {
    try {
      const parsed = JSON.parse(raw.metadata)
      if (parsed && typeof parsed === 'object') {
        metadata = parsed as Record<string, unknown>
      }
    } catch {
      metadata = undefined
    }
  }

  return {
    id,
    display_name:
      typeof raw.display_name === 'string' && raw.display_name.trim().length > 0
        ? raw.display_name
        : undefined,
    description:
      typeof raw.description === 'string' && raw.description.trim().length > 0
        ? raw.description
        : undefined,
    category:
      typeof raw.category === 'string' && raw.category.trim().length > 0 ? raw.category : undefined,
    default_scope:
      typeof raw.default_scope === 'string' && raw.default_scope.trim().length > 0
        ? raw.default_scope
        : undefined,
    module: typeof raw.module === 'string' && raw.module.trim().length > 0 ? raw.module : undefined,
    depends_on: normalizeStringArrayPayload(raw.depends_on),
    implies: normalizeStringArrayPayload(raw.implies),
    metadata,
  }
}

function transformProtocol(raw: ProtocolResponse): Protocol {
  const permissions =
    raw.permissions
      ?.map((permission) => transformProtocolPermission(permission))
      .filter((permission): permission is ProtocolPermission => permission !== null) ?? []

  return {
    id: raw.id,
    name: raw.name,
    module: raw.module,
    description: raw.description,
    category: raw.category ?? 'terminal',
    icon: raw.icon,
    defaultPort: raw.default_port,
    sortOrder: raw.sort_order,
    features: raw.features ?? [],
    capabilities: normalizeCapabilities(raw.capabilities),
    driverEnabled: raw.driver_enabled,
    configEnabled: raw.config_enabled,
    available: raw.available,
    permissions,
  }
}

interface ProtocolListResponse {
  protocols?: ProtocolResponse[]
  count?: number
}

function transformProtocolList(
  payload: ProtocolListResponse | ProtocolResponse[]
): ProtocolListResult {
  const protocolsArray = Array.isArray(payload)
    ? payload
    : Array.isArray(payload?.protocols)
      ? payload.protocols
      : []

  const transformed = protocolsArray.map(transformProtocol)
  const count =
    !Array.isArray(payload) && typeof payload?.count === 'number'
      ? payload.count
      : transformed.length

  return {
    data: transformed,
    count,
  }
}

export async function fetchProtocols(): Promise<ProtocolListResult> {
  const response =
    await apiClient.get<ApiResponse<ProtocolListResponse | ProtocolResponse[]>>(PROTOCOLS_ENDPOINT)
  const data = unwrapResponse(response)
  return transformProtocolList(data)
}

export async function fetchAvailableProtocols(): Promise<ProtocolListResult> {
  const response = await apiClient.get<ApiResponse<ProtocolListResponse | ProtocolResponse[]>>(
    PROTOCOLS_AVAILABLE_ENDPOINT
  )
  const data = unwrapResponse(response)
  return transformProtocolList(data)
}

export async function fetchProtocolPermissions(protocolId: string): Promise<ProtocolPermission[]> {
  const response = await apiClient.get<ApiResponse<{ permissions?: ProtocolPermissionResponse[] }>>(
    PROTOCOL_PERMISSIONS_ENDPOINT(protocolId)
  )
  const data = unwrapResponse(response)
  const permissions = Array.isArray(data.permissions) ? data.permissions : []
  return permissions
    .map((permission) => transformProtocolPermission(permission))
    .filter((permission): permission is ProtocolPermission => permission !== null)
}
