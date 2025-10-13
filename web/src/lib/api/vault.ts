import { apiClient } from './client'
import { unwrapResponse } from './http'
import type { ApiResponse } from '@/types/api'
import type {
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
  return unwrapResponse(response)
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
