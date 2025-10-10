import type { ApiResponse } from '@/types/api'
import type { Protocol, ProtocolCapabilities, ProtocolListResult } from '@/types/protocols'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const PROTOCOLS_ENDPOINT = '/protocols'
const PROTOCOLS_AVAILABLE_ENDPOINT = '/protocols/available'

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

function transformProtocol(raw: ProtocolResponse): Protocol {
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
