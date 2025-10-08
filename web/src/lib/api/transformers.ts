import type {
  AuthTokens,
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
