import type { ApiResponse } from '@/types/api'
import type { Protocol, ProtocolCapabilities } from '@/types/protocols'
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

export async function fetchProtocols(): Promise<Protocol[]> {
  const response = await apiClient.get<ApiResponse<ProtocolResponse[]>>(PROTOCOLS_ENDPOINT)
  const data = unwrapResponse(response)
  return data.map(transformProtocol)
}

export async function fetchAvailableProtocols(): Promise<Protocol[]> {
  const response = await apiClient.get<ApiResponse<ProtocolResponse[]>>(
    PROTOCOLS_AVAILABLE_ENDPOINT
  )
  const data = unwrapResponse(response)
  return data.map(transformProtocol)
}
