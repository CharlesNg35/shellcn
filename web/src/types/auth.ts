export type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated' | 'mfa_required'

export interface AuthRoleSummary {
  id: string
  name: string
  description?: string
}

export interface AuthTeamSummary {
  id: string
  name: string
}

export interface AuthUser {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  is_root: boolean
  is_active: boolean
  teams?: AuthTeamSummary[]
  roles?: AuthRoleSummary[]
  permissions?: import('@/constants/permissions').PermissionId[]
  mfa_enrolled?: boolean
  last_login_at?: string
}

export interface AuthTokens {
  accessToken: string
  refreshToken: string
  expiresIn: number
  expiresAt: number
}

export interface LoginCredentials {
  identifier: string
  password: string
  mfa_token?: string
  remember_device?: boolean
}

export interface LoginResponsePayload {
  access_token: string
  refresh_token: string
  expires_in: number
  user?: AuthUser
}

export interface RefreshResponsePayload {
  access_token: string
  refresh_token: string
  expires_in: number
  user?: AuthUser
}

export interface MfaChallenge {
  challenge_id: string
  method: string
  methods?: string[]
  expires_at?: string
  details?: Record<string, unknown>
}

export interface LoginResult {
  tokens?: AuthTokens
  user?: AuthUser
  mfaRequired?: boolean
  challenge?: MfaChallenge
}

export interface VerifyMfaPayload {
  challenge_id: string
  mfa_token: string
  remember_device?: boolean
}

export type SetupStatus = 'pending' | 'complete'

export interface SetupStatusPayload {
  status: SetupStatus
  message?: string
}

export interface SetupInitializePayload {
  username: string
  email: string
  password: string
  first_name?: string
  last_name?: string
}

export interface SetupInitializeResponse {
  user: AuthUser
  message?: string
}

export interface AuthProviderMetadata {
  type: 'oidc' | 'saml' | 'ldap' | 'local' | string
  name: string
  description?: string
  icon?: string
  enabled: boolean
  login_url?: string
  allow_registration?: boolean
  require_email_verification?: boolean
  allow_password_reset?: boolean
}

export interface PasswordResetRequestPayload {
  identifier?: string
  email?: string
}

export interface PasswordResetConfirmPayload {
  token: string
  password: string
  confirm_password?: string
}

export interface PasswordResetResponse {
  message?: string
}
