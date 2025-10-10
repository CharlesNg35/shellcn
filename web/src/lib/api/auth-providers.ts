import type { AxiosRequestConfig } from 'axios'
import type { ApiResponse } from '@/types/api'
import {
  type AuthProviderDetails,
  type AuthProviderPublicRecord,
  type AuthProviderRecord,
  type AuthProviderType,
  type LDAPProviderConfig,
  type LocalAuthSettings,
  type OIDCProviderConfig,
  type SAMLProviderConfig,
} from '@/types/auth-providers'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const AUTH_PROVIDERS_BASE = '/auth/providers'

interface AuthProviderResponse {
  id: string
  type: AuthProviderType
  name: string
  enabled: boolean
  allow_registration?: boolean
  require_email_verification?: boolean
  allow_password_reset?: boolean
  description?: string
  icon?: string
  created_by?: string
  created_at?: string
  updated_at?: string
}

interface PublicAuthProviderResponse {
  type: AuthProviderType
  name: string
  description?: string
  icon?: string
  enabled: boolean
  allow_registration?: boolean
  require_email_verification?: boolean
  allow_password_reset?: boolean
  flow: 'password' | 'redirect' | (string & {})
}

interface UpdateResponse {
  updated?: boolean
  ok?: boolean
}

interface OIDCConfigResponse {
  issuer: string
  client_id: string
  client_secret: string
  redirect_url: string
  scopes?: string[]
}

interface SAMLConfigResponse {
  metadata_url?: string
  entity_id: string
  sso_url: string
  acs_url: string
  certificate: string
  private_key: string
  attribute_mapping?: Record<string, string>
}

interface LDAPConfigResponse {
  host: string
  port: number
  base_dn: string
  bind_dn: string
  bind_password: string
  user_filter: string
  use_tls: boolean
  skip_verify: boolean
  attribute_mapping?: Record<string, string>
}

interface AuthProviderDetailResponse<TConfig = unknown> {
  provider: AuthProviderResponse
  config?: TConfig
}

type ProviderConfigFor<T extends AuthProviderType> = T extends 'oidc'
  ? OIDCProviderConfig
  : T extends 'saml'
    ? SAMLProviderConfig
    : T extends 'ldap'
      ? LDAPProviderConfig
      : undefined

function transformAuthProvider(raw: AuthProviderResponse): AuthProviderRecord {
  return {
    id: raw.id,
    type: raw.type,
    name: raw.name,
    enabled: raw.enabled,
    allowRegistration: Boolean(raw.allow_registration),
    requireEmailVerification: Boolean(raw.require_email_verification),
    allowPasswordReset: Boolean(raw.allow_password_reset),
    description: raw.description,
    icon: raw.icon,
    createdBy: raw.created_by,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  }
}

function transformPublicAuthProvider(raw: PublicAuthProviderResponse): AuthProviderPublicRecord {
  return {
    type: raw.type,
    name: raw.name,
    description: raw.description,
    icon: raw.icon,
    enabled: raw.enabled,
    allowRegistration: Boolean(raw.allow_registration),
    requireEmailVerification: Boolean(raw.require_email_verification),
    allowPasswordReset: Boolean(raw.allow_password_reset),
    flow: raw.flow,
  }
}

function transformOIDCConfig(raw: OIDCConfigResponse | undefined): OIDCProviderConfig | undefined {
  if (!raw) {
    return undefined
  }
  return {
    issuer: raw.issuer,
    clientId: raw.client_id,
    clientSecret: raw.client_secret,
    redirectUrl: raw.redirect_url,
    scopes: raw.scopes ?? [],
  }
}

function transformSAMLConfig(raw: SAMLConfigResponse | undefined): SAMLProviderConfig | undefined {
  if (!raw) {
    return undefined
  }
  return {
    metadataUrl: raw.metadata_url,
    entityId: raw.entity_id,
    ssoUrl: raw.sso_url,
    acsUrl: raw.acs_url,
    certificate: raw.certificate,
    privateKey: raw.private_key,
    attributeMapping: raw.attribute_mapping ?? {},
  }
}

function transformLDAPConfig(raw: LDAPConfigResponse | undefined): LDAPProviderConfig | undefined {
  if (!raw) {
    return undefined
  }
  return {
    host: raw.host,
    port: raw.port,
    baseDn: raw.base_dn,
    bindDn: raw.bind_dn,
    bindPassword: raw.bind_password,
    userFilter: raw.user_filter,
    useTls: raw.use_tls,
    skipVerify: raw.skip_verify,
    attributeMapping: raw.attribute_mapping ?? {},
  }
}

async function getProviders<T>(
  path: string,
  transform: (raw: AuthProviderResponse) => AuthProviderRecord,
  config?: AxiosRequestConfig
): Promise<T> {
  const response = await apiClient.get<ApiResponse<AuthProviderResponse[]>>(path, config)
  const data = unwrapResponse(response)
  return data.map(transform) as unknown as T
}

export async function getAllAuthProviders(): Promise<AuthProviderRecord[]> {
  return getProviders(`${AUTH_PROVIDERS_BASE}/all`, transformAuthProvider)
}

export async function getEnabledAuthProviders(): Promise<AuthProviderRecord[]> {
  return getProviders(`${AUTH_PROVIDERS_BASE}/enabled`, transformAuthProvider)
}

export async function getPublicAuthProviders(): Promise<AuthProviderPublicRecord[]> {
  const response =
    await apiClient.get<ApiResponse<PublicAuthProviderResponse[]>>(AUTH_PROVIDERS_BASE)
  const data = unwrapResponse(response)
  return data.map(transformPublicAuthProvider)
}

export async function getAuthProviderDetails<T extends AuthProviderType>(
  providerType: T
): Promise<AuthProviderDetails<ProviderConfigFor<T>>> {
  const response = await apiClient.get<ApiResponse<AuthProviderDetailResponse>>(
    `${AUTH_PROVIDERS_BASE}/${providerType}`
  )
  const data = unwrapResponse(response)

  const provider = transformAuthProvider(data.provider)

  let config: OIDCProviderConfig | SAMLProviderConfig | LDAPProviderConfig | undefined

  switch (provider.type) {
    case 'oidc':
      config = transformOIDCConfig(data.config as OIDCConfigResponse | undefined)
      break
    case 'saml':
      config = transformSAMLConfig(data.config as SAMLConfigResponse | undefined)
      break
    case 'ldap':
      config = transformLDAPConfig(data.config as LDAPConfigResponse | undefined)
      break
    default:
      config = undefined
  }

  return {
    provider,
    config,
  } as AuthProviderDetails<ProviderConfigFor<T>>
}

export async function updateLocalAuthSettings(payload: LocalAuthSettings): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/local/settings`,
    {
      allow_registration: payload.allowRegistration,
      require_email_verification: payload.requireEmailVerification,
      allow_password_reset: payload.allowPasswordReset,
    }
  )
  unwrapResponse(response)
}

export async function configureOIDCProvider(payload: {
  enabled: boolean
  allowRegistration: boolean
  config: OIDCProviderConfig
}): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/oidc/configure`,
    {
      enabled: payload.enabled,
      allow_registration: payload.allowRegistration,
      config: {
        issuer: payload.config.issuer,
        client_id: payload.config.clientId,
        client_secret: payload.config.clientSecret,
        redirect_url: payload.config.redirectUrl,
        scopes: payload.config.scopes,
      },
    }
  )
  unwrapResponse(response)
}

export async function configureSAMLProvider(payload: {
  enabled: boolean
  allowRegistration: boolean
  config: SAMLProviderConfig
}): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/saml/configure`,
    {
      enabled: payload.enabled,
      allow_registration: payload.allowRegistration,
      config: {
        metadata_url: payload.config.metadataUrl,
        entity_id: payload.config.entityId,
        sso_url: payload.config.ssoUrl,
        acs_url: payload.config.acsUrl,
        certificate: payload.config.certificate,
        private_key: payload.config.privateKey,
        attribute_mapping: payload.config.attributeMapping,
      },
    }
  )
  unwrapResponse(response)
}

export async function configureLDAPProvider(payload: {
  enabled: boolean
  allowRegistration: boolean
  config: LDAPProviderConfig
}): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/ldap/configure`,
    {
      enabled: payload.enabled,
      allow_registration: payload.allowRegistration,
      config: {
        host: payload.config.host,
        port: payload.config.port,
        base_dn: payload.config.baseDn,
        bind_dn: payload.config.bindDn,
        bind_password: payload.config.bindPassword,
        user_filter: payload.config.userFilter,
        use_tls: payload.config.useTls,
        skip_verify: payload.config.skipVerify,
        attribute_mapping: payload.config.attributeMapping,
      },
    }
  )
  unwrapResponse(response)
}

export async function setAuthProviderEnabled(
  providerType: AuthProviderType,
  enabled: boolean
): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/${providerType}/enable`,
    {
      enabled,
    }
  )
  unwrapResponse(response)
}

export async function testAuthProviderConnection(providerType: AuthProviderType): Promise<void> {
  const response = await apiClient.post<ApiResponse<UpdateResponse>>(
    `${AUTH_PROVIDERS_BASE}/${providerType}/test`
  )
  unwrapResponse(response)
}

export const authProvidersApi = {
  getAll: getAllAuthProviders,
  getEnabled: getEnabledAuthProviders,
  getPublic: getPublicAuthProviders,
  getDetails: getAuthProviderDetails,
  updateLocalSettings: updateLocalAuthSettings,
  configureOIDC: configureOIDCProvider,
  configureSAML: configureSAMLProvider,
  configureLDAP: configureLDAPProvider,
  setEnabled: setAuthProviderEnabled,
  testConnection: testAuthProviderConnection,
}
