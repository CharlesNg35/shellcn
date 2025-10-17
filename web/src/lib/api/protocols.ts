import type { ApiResponse } from '@/types/api'
import type {
  Protocol,
  ProtocolCapabilities,
  ProtocolListResult,
  ProtocolPermission,
  ConnectionTemplate,
  ConnectionTemplateField,
  ConnectionTemplateSection,
  ConnectionTemplateFieldType,
  ConnectionTemplateBinding,
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
  connection_template_version?: string | null
  identity_required?: boolean
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
    display_name: withStringValue(raw.display_name),
    description: withStringValue(raw.description),
    category: withStringValue(raw.category),
    default_scope: withStringValue(raw.default_scope),
    module: withStringValue(raw.module),
    depends_on: normalizeStringArrayPayload(raw.depends_on),
    implies: normalizeStringArrayPayload(raw.implies),
    metadata,
  }
}

function withStringValue(value: string | undefined): string | undefined {
  return typeof value === 'string' && value.trim().length > 0 ? value : undefined
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
    connectionTemplateVersion: raw.connection_template_version ?? undefined,
    identityRequired: Boolean(raw.identity_required),
    permissions,
  }
}

interface ProtocolListResponse {
  protocols?: ProtocolResponse[]
  count?: number
}

interface ConnectionTemplateResponse {
  driver_id: string
  version: string
  display_name: string
  description?: string
  sections?: ConnectionTemplateSectionResponse[] | null
  metadata?: Record<string, unknown> | string | null
}

interface ConnectionTemplateSectionResponse {
  id: string
  label: string
  description?: string
  fields?: ConnectionTemplateFieldResponse[] | null
  metadata?: Record<string, unknown> | string | null
}

interface ConnectionTemplateFieldResponse {
  key: string
  label: string
  type: string
  required: boolean
  default?: unknown
  placeholder?: string
  help_text?: string
  options?: ConnectionTemplateOptionResponse[] | null
  binding?: ConnectionTemplateBindingResponse | null
  validation?: Record<string, unknown> | string | null
  dependencies?: ConnectionTemplateDependencyResponse[] | null
  metadata?: Record<string, unknown> | string | null
}

interface ConnectionTemplateOptionResponse {
  value: string
  label: string
}

interface ConnectionTemplateBindingResponse {
  target?: string
  path?: string
  index?: number
  property?: string
}

interface ConnectionTemplateDependencyResponse {
  field: string
  equals?: unknown
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

const PROTOCOL_TEMPLATE_ENDPOINT = (protocolId: string) =>
  `/protocols/${protocolId}/connection-template`

function parseRecordPayload(
  value?: Record<string, unknown> | string | null
): Record<string, unknown> | undefined {
  if (!value) {
    return undefined
  }
  if (typeof value === 'object') {
    return value
  }
  if (typeof value === 'string' && value.trim().length > 0) {
    try {
      const parsed = JSON.parse(value)
      if (parsed && typeof parsed === 'object') {
        return parsed as Record<string, unknown>
      }
    } catch {
      return undefined
    }
  }
  return undefined
}

function transformConnectionTemplateField(
  field: ConnectionTemplateFieldResponse
): ConnectionTemplateField {
  return {
    key: field.key,
    label: field.label,
    type: field.type as ConnectionTemplateFieldType,
    required: Boolean(field.required),
    default: field.default,
    placeholder: field.placeholder,
    helpText: field.help_text,
    options: (field.options ?? undefined)?.map((option) => ({
      value: option.value,
      label: option.label,
    })),
    binding: field.binding
      ? {
          target: (field.binding.target ?? 'settings') as ConnectionTemplateBinding['target'],
          path: field.binding.path ?? undefined,
          index: field.binding.index ?? undefined,
          property: field.binding.property ?? undefined,
        }
      : undefined,
    validation: parseRecordPayload(field.validation),
    dependencies: (field.dependencies ?? undefined)?.map((dependency) => ({
      field: dependency.field,
      equals: dependency.equals,
    })),
    metadata: parseRecordPayload(field.metadata),
  }
}

function transformConnectionTemplateSection(
  section: ConnectionTemplateSectionResponse
): ConnectionTemplateSection {
  return {
    id: section.id,
    label: section.label,
    description: section.description,
    fields: (section.fields ?? []).map(transformConnectionTemplateField),
    metadata: parseRecordPayload(section.metadata),
  }
}

function transformConnectionTemplateResponse(
  raw?: ConnectionTemplateResponse | null
): ConnectionTemplate | null {
  if (!raw) {
    return null
  }
  return {
    driverId: raw.driver_id,
    version: raw.version,
    displayName: raw.display_name,
    description: raw.description,
    sections: (raw.sections ?? []).map(transformConnectionTemplateSection),
    metadata: parseRecordPayload(raw.metadata),
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

export async function fetchConnectionTemplate(
  protocolId: string
): Promise<ConnectionTemplate | null> {
  const response = await apiClient.get<
    ApiResponse<{ template?: ConnectionTemplateResponse | null }>
  >(PROTOCOL_TEMPLATE_ENDPOINT(protocolId))
  const data = unwrapResponse(response)
  if (!data || typeof data !== 'object') {
    return null
  }
  return transformConnectionTemplateResponse(
    (data as { template?: ConnectionTemplateResponse | null }).template
  )
}
