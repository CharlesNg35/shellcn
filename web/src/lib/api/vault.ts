import { apiClient } from './client'
import { unwrapResponse } from './http'
import type { ApiResponse } from '@/types/api'
import type {
  CredentialField,
  CredentialFieldComparable,
  CredentialFieldMetadata,
  CredentialFieldType,
  CredentialFieldVisibilityRule,
  CredentialTemplateRecord,
  IdentityCreatePayload,
  IdentityListParams,
  IdentityRecord,
  IdentitySharePayload,
  IdentityShareRecord,
  IdentityUpdatePayload,
} from '@/types/vault'

const VAULT_IDENTITIES_ENDPOINT = '/vault/identities'
const VAULT_TEMPLATES_ENDPOINT = '/vault/templates'
const VAULT_SHARES_ENDPOINT = '/vault/shares'
const ALLOWED_FIELD_TYPES: CredentialFieldType[] = [
  'string',
  'secret',
  'file',
  'enum',
  'boolean',
  'number',
]

type IdentityResponse = Omit<IdentityRecord, 'metadata' | 'payload' | 'shares'> & {
  metadata?: Record<string, unknown> | null
  payload?: Record<string, unknown> | null
  shares?: IdentityShareRecord[] | null
}

function normalizeIdentity(raw: IdentityResponse): IdentityRecord {
  return {
    ...raw,
    metadata: raw.metadata ?? undefined,
    payload: raw.payload ?? undefined,
    shares: raw.shares ?? [],
  }
}

function buildIdentityQueryParams(
  params?: IdentityListParams
): Record<string, unknown> | undefined {
  if (!params) {
    return undefined
  }

  const query: Record<string, unknown> = {}

  if (params.scope && params.scope !== 'all') {
    query.scope = params.scope
  }

  if (params.protocol_id) {
    query.protocol_id = params.protocol_id
  }

  if (typeof params.include_connection_scoped === 'boolean') {
    query.include_connection_scoped = params.include_connection_scoped
  }

  return query
}

export async function fetchIdentities(params?: IdentityListParams): Promise<IdentityRecord[]> {
  const response = await apiClient.get<ApiResponse<IdentityResponse[]>>(VAULT_IDENTITIES_ENDPOINT, {
    params: buildIdentityQueryParams(params),
  })

  const payload = unwrapResponse(response)
  return payload.map(normalizeIdentity)
}

export interface FetchIdentityOptions {
  includePayload?: boolean
}

export async function fetchIdentity(
  identityId: string,
  options?: FetchIdentityOptions
): Promise<IdentityRecord> {
  const query: Record<string, string> = {}
  if (options?.includePayload) {
    query.include = 'payload'
  }

  const response = await apiClient.get<ApiResponse<IdentityResponse>>(
    `${VAULT_IDENTITIES_ENDPOINT}/${identityId}`,
    {
      params: query,
    }
  )

  return normalizeIdentity(unwrapResponse(response))
}

export async function createIdentity(payload: IdentityCreatePayload): Promise<IdentityRecord> {
  const response = await apiClient.post<ApiResponse<IdentityResponse>>(
    VAULT_IDENTITIES_ENDPOINT,
    payload
  )
  return normalizeIdentity(unwrapResponse(response))
}

export async function updateIdentity(
  identityId: string,
  payload: IdentityUpdatePayload
): Promise<IdentityRecord> {
  const response = await apiClient.patch<ApiResponse<IdentityResponse>>(
    `${VAULT_IDENTITIES_ENDPOINT}/${identityId}`,
    payload
  )
  return normalizeIdentity(unwrapResponse(response))
}

export async function deleteIdentity(identityId: string): Promise<void> {
  await apiClient.delete(`${VAULT_IDENTITIES_ENDPOINT}/${identityId}`)
}

export async function createIdentityShare(
  identityId: string,
  payload: IdentitySharePayload
): Promise<IdentityShareRecord> {
  const response = await apiClient.post<ApiResponse<IdentityShareRecord>>(
    `${VAULT_IDENTITIES_ENDPOINT}/${identityId}/shares`,
    payload
  )
  return unwrapResponse(response)
}

export async function deleteIdentityShare(shareId: string): Promise<void> {
  await apiClient.delete(`${VAULT_SHARES_ENDPOINT}/${shareId}`)
}

export async function fetchCredentialTemplates(): Promise<CredentialTemplateRecord[]> {
  const response =
    await apiClient.get<ApiResponse<CredentialTemplateRecord[]>>(VAULT_TEMPLATES_ENDPOINT)
  const payload = unwrapResponse(response) as CredentialTemplateResponse[]
  return payload.map(normalizeCredentialTemplate)
}

export const vaultApi = {
  listIdentities: fetchIdentities,
  getIdentity: fetchIdentity,
  createIdentity,
  updateIdentity,
  deleteIdentity,
  createIdentityShare,
  deleteIdentityShare,
  listTemplates: fetchCredentialTemplates,
}

type CredentialTemplateResponse = Omit<CredentialTemplateRecord, 'fields'> & {
  fields: Array<Record<string, unknown>>
}

function normalizeCredentialTemplate(raw: CredentialTemplateResponse): CredentialTemplateRecord {
  const metadata = normalizeTemplateMetadata(raw.metadata)
  const compatibleProtocols = Array.isArray(raw.compatible_protocols)
    ? raw.compatible_protocols.filter(isNonEmptyString)
    : []

  return {
    ...raw,
    description: typeof raw.description === 'string' ? raw.description : undefined,
    metadata,
    compatible_protocols: compatibleProtocols,
    fields: Array.isArray(raw.fields)
      ? raw.fields.map((field, index) => normalizeCredentialField(field, index))
      : [],
  }
}

function normalizeCredentialField(raw: Record<string, unknown>, index: number): CredentialField {
  const normalized = normalizeKeyedObject(raw)

  const resolvedName = pickString(normalized, ['name', 'key', 'field', 'identifier', 'id'])
  const key = pickString(normalized, ['key', 'name', 'identifier', 'id'])
  const resolvedLabel =
    pickString(normalized, ['label', 'display_name', 'title']) ??
    resolvedName ??
    `Field ${index + 1}`
  const typeValue = pickString(normalized, ['type', 'field_type']) ?? 'string'

  const description = pickString(normalized, ['description', 'help_text', 'summary', 'hint'])
  const placeholder = pickString(normalized, ['placeholder', 'prompt'])
  const defaultValue = pickValue(normalized, ['default_value', 'default'])
  const requiredValue = pickValue(normalized, ['required', 'is_required', 'mandatory'])
  const inputModesValue = pickValue(normalized, ['input_modes', 'input_mode', 'modes'])
  const optionsValue = pickValue(normalized, ['options', 'choices', 'values'])
  const validationValue = pickValue(normalized, ['validation', 'rules'])
  const metadataValue = pickValue(normalized, ['metadata'])

  const inputModes = normalizeStringArray(inputModesValue)
  const options = normalizeOptions(optionsValue)
  const metadata = normalizeFieldMetadata(metadataValue)
  const validation = isRecord(validationValue) ? validationValue : undefined

  const field: CredentialField = {
    name: (resolvedName ?? key ?? `field_${index + 1}`).trim(),
    type: normalizeFieldType(typeValue),
    label: resolvedLabel?.trim(),
    description: description?.trim(),
    required: normalizeBoolean(requiredValue),
    placeholder: placeholder?.trim(),
    default_value: defaultValue,
    input_modes: inputModes,
    options,
    metadata,
  }

  if (validation) {
    field.validation = validation
  }
  if (key && key !== field.name) {
    field.key = key
  }

  return field
}

function normalizeOptions(value: unknown): Array<string | Record<string, unknown>> | undefined {
  if (!Array.isArray(value)) {
    return undefined
  }
  return value
    .map((option) => {
      if (typeof option === 'string') {
        return option
      }
      if (!isRecord(option)) {
        return undefined
      }

      const normalized = normalizeKeyedObject(option)
      const rawValue = pickValue(normalized, ['value', 'key', 'id'])
      const rawLabel = pickValue(normalized, ['label', 'name', 'display_name'])

      const optionRecord: Record<string, unknown> = {
        ...Object.fromEntries(Object.entries(option).map(([key, val]) => [toSnakeCase(key), val])),
      }

      if (rawValue !== undefined) {
        optionRecord.value = rawValue
      }
      if (rawLabel !== undefined) {
        optionRecord.label = rawLabel
      } else if (rawValue !== undefined) {
        optionRecord.label = typeof rawValue === 'string' ? rawValue : String(rawValue)
      }

      return optionRecord
    })
    .filter((option): option is string | Record<string, unknown> => Boolean(option))
}

function normalizeFieldMetadata(value: unknown): CredentialFieldMetadata | undefined {
  if (!isRecord(value)) {
    return undefined
  }

  const metadata = normalizeKeyedObject(value) as CredentialFieldMetadata

  if ('visibility' in metadata) {
    metadata.visibility = normalizeVisibility(metadata.visibility)
  }

  if ('required_when' in metadata) {
    metadata.required_when = normalizeVisibility(metadata.required_when)
  }

  if ('allow_file_import' in metadata) {
    metadata.allow_file_import = Boolean(metadata.allow_file_import)
  }

  if ('rows' in metadata && typeof (metadata.rows as unknown) !== 'number') {
    metadata.rows = undefined
  }

  if ('hint' in metadata && typeof metadata.hint !== 'string') {
    metadata.hint = undefined
  }

  return metadata
}

function normalizeTemplateMetadata(value: unknown): Record<string, unknown> | undefined {
  if (!isRecord(value)) {
    return undefined
  }

  const metadata = normalizeKeyedObject(value)

  if ('sections' in metadata && Array.isArray(metadata.sections)) {
    metadata.sections = metadata.sections
      .map((section) => (isRecord(section) ? normalizeKeyedObject(section) : null))
      .filter(Boolean)
  }

  if ('defaults' in metadata && isRecord(metadata.defaults)) {
    metadata.defaults = normalizeKeyedObject(metadata.defaults)
  }

  return metadata
}

function normalizeVisibility(
  raw: unknown
): CredentialFieldVisibilityRule | CredentialFieldVisibilityRule[] | undefined {
  if (Array.isArray(raw)) {
    const rules = raw
      .map((rule) => (isRecord(rule) ? normalizeVisibilityRule(rule) : undefined))
      .filter((rule): rule is CredentialFieldVisibilityRule => Boolean(rule))
    return rules.length ? rules : undefined
  }

  if (isRecord(raw)) {
    return normalizeVisibilityRule(raw)
  }

  return undefined
}

function normalizeVisibilityRule(
  raw: Record<string, unknown>
): CredentialFieldVisibilityRule | undefined {
  const normalized = normalizeKeyedObject(raw)
  const field =
    typeof normalized.field === 'string' && normalized.field.trim().length > 0
      ? normalized.field.trim()
      : undefined

  if (!field) {
    return undefined
  }

  const rule: CredentialFieldVisibilityRule = { field }

  if ('equals' in normalized) {
    const equals = normalizeComparableArray(normalized.equals)
    if (equals && equals.length > 0) {
      rule.equals = equals.length === 1 ? equals[0] : equals
    }
  }

  if ('not_equals' in normalized) {
    const notEquals = normalizeComparableArray(normalized.not_equals)
    if (notEquals && notEquals.length > 0) {
      rule.not_equals = notEquals.length === 1 ? notEquals[0] : notEquals
    }
  }

  if ('in' in normalized) {
    const values = normalizeComparableArray(normalized.in)
    if (values && values.length > 0) {
      rule.in = values
    }
  }

  if ('not_in' in normalized) {
    const values = normalizeComparableArray(normalized.not_in)
    if (values && values.length > 0) {
      rule.not_in = values
    }
  }

  if ('mode' in normalized && typeof normalized.mode === 'string') {
    const normalizedMode = normalized.mode.toLowerCase()
    if (normalizedMode === 'all' || normalizedMode === 'any') {
      rule.mode = normalizedMode
    }
  }

  if ('exists' in normalized) {
    rule.exists = Boolean(normalized.exists)
  }

  if ('not_exists' in normalized) {
    rule.not_exists = Boolean(normalized.not_exists)
  }

  if ('truthy' in normalized) {
    rule.truthy = Boolean(normalized.truthy)
  }

  if ('falsy' in normalized) {
    rule.falsy = Boolean(normalized.falsy)
  }

  return rule
}

function normalizeComparableArray(value: unknown): CredentialFieldComparable[] | undefined {
  if (Array.isArray(value)) {
    return value
      .map((item) => {
        if (typeof item === 'string' || typeof item === 'number' || typeof item === 'boolean') {
          return item
        }
        return undefined
      })
      .filter((item): item is string | number | boolean => item !== undefined)
  }

  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return [value]
  }

  return undefined
}

function normalizeBoolean(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['true', '1', 'yes', 'y'].includes(normalized)) {
      return true
    }
    if (['false', '0', 'no', 'n'].includes(normalized)) {
      return false
    }
  }
  if (typeof value === 'number') {
    if (value === 1) return true
    if (value === 0) return false
  }
  return undefined
}

function normalizeStringArray(value: unknown): string[] | undefined {
  if (Array.isArray(value)) {
    return value
      .map((item) => (typeof item === 'string' ? item.trim() : undefined))
      .filter(isNonEmptyString)
  }
  if (typeof value === 'string') {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(isNonEmptyString)
  }
  return undefined
}

function normalizeKeyedObject(source: Record<string, unknown>): Record<string, unknown> {
  return Object.entries(source).reduce<Record<string, unknown>>((acc, [key, value]) => {
    const normalizedKey = toSnakeCase(key)
    if (!(normalizedKey in acc)) {
      acc[normalizedKey] = value
    }
    return acc
  }, {})
}

function pickString(source: Record<string, unknown>, candidates: string[]): string | undefined {
  for (const candidate of candidates) {
    const normalizedKey = toSnakeCase(candidate)
    const raw = source[normalizedKey]
    if (typeof raw === 'string') {
      const trimmed = raw.trim()
      if (trimmed.length > 0) {
        return trimmed
      }
    }
  }
  return undefined
}

function pickValue(source: Record<string, unknown>, candidates: string[]): unknown {
  for (const candidate of candidates) {
    const normalizedKey = toSnakeCase(candidate)
    if (normalizedKey in source) {
      return source[normalizedKey]
    }
  }
  return undefined
}

function normalizeFieldType(value: string): CredentialFieldType {
  const normalized = value.trim().toLowerCase()
  if ((ALLOWED_FIELD_TYPES as string[]).includes(normalized)) {
    return normalized as CredentialFieldType
  }
  return 'string'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isNonEmptyString(value: string | undefined): value is string {
  return Boolean(value && value.length > 0)
}

function toSnakeCase(input: string): string {
  return input
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .replace(/[\s-]+/g, '_')
    .toLowerCase()
}
