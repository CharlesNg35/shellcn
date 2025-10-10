export type AuthProviderType = 'local' | 'invite' | 'oidc' | 'saml' | 'ldap' | (string & {})

export interface AuthProviderRecord {
  id: string
  type: AuthProviderType
  name: string
  enabled: boolean
  allowRegistration: boolean
  requireEmailVerification: boolean
  allowPasswordReset: boolean
  description?: string
  icon?: string
  createdBy?: string
  createdAt?: string
  updatedAt?: string
}

export interface AuthProviderPublicRecord {
  type: AuthProviderType
  name: string
  description?: string
  icon?: string
  enabled: boolean
  allowRegistration: boolean
  requireEmailVerification: boolean
  allowPasswordReset: boolean
  flow: 'password' | 'redirect' | (string & {})
}

export interface LocalAuthSettings {
  allowRegistration: boolean
  requireEmailVerification: boolean
  allowPasswordReset: boolean
}

export interface InviteAuthSettings {
  enabled: boolean
  requireEmailVerification: boolean
}

export interface OIDCProviderConfig {
  issuer: string
  clientId: string
  clientSecret: string
  redirectUrl: string
  scopes: string[]
}

export interface SAMLProviderConfig {
  metadataUrl?: string
  entityId: string
  ssoUrl: string
  acsUrl: string
  certificate: string
  privateKey: string
  attributeMapping: Record<string, string>
}

export interface LDAPProviderConfig {
  host: string
  port: number
  baseDn: string
  bindDn: string
  bindPassword: string
  userFilter: string
  useTls: boolean
  skipVerify: boolean
  attributeMapping: Record<string, string>
}

export type AuthProviderConfigMap = {
  oidc: OIDCProviderConfig
  saml: SAMLProviderConfig
  ldap: LDAPProviderConfig
}

export interface AuthProviderDetails<TConfig = unknown> {
  provider: AuthProviderRecord
  config?: TConfig
}
