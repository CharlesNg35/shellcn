import type {
  AuthTokens,
  AuthUser,
  LoginResponsePayload,
  RefreshResponsePayload,
} from '@/types/auth'

type TokenSource = LoginResponsePayload | RefreshResponsePayload

export function toAuthTokens(payload: TokenSource): AuthTokens | null {
  if (!payload.access_token || !payload.refresh_token || !payload.expires_in) {
    return null
  }

  const expiresAt = Date.now() + payload.expires_in * 1000

  return {
    accessToken: payload.access_token,
    refreshToken: payload.refresh_token,
    expiresIn: payload.expires_in,
    expiresAt,
  }
}

export function normalizeAuthProvider(provider?: string | null): string {
  if (!provider) {
    return 'local'
  }
  const normalized = provider.trim().toLowerCase()
  if (normalized === '') {
    return 'local'
  }
  return normalized
}

export function transformAuthUser(user?: AuthUser | null): AuthUser | null {
  if (!user) {
    return user ?? null
  }
  return {
    ...user,
    auth_provider: normalizeAuthProvider(user.auth_provider),
  }
}
