export interface JwtClaims {
  [key: string]: unknown
  sub?: string
  sid?: string
  jti?: string
  uid?: string
  session_id?: string
}

function base64UrlDecode(segment: string): string {
  let normalized = segment.replace(/-/g, '+').replace(/_/g, '/')
  const padding = normalized.length % 4
  if (padding) {
    normalized += '='.repeat(4 - padding)
  }

  if (typeof globalThis.atob === 'function') {
    return globalThis.atob(normalized)
  }

  const globalObject = globalThis as Record<string, unknown>
  const bufferCtor = globalObject.Buffer as
    | { from(data: string, encoding: string): { toString(encoding: string): string } }
    | undefined

  if (bufferCtor?.from) {
    return bufferCtor.from(normalized, 'base64').toString('utf-8')
  }

  throw new Error('No base64 decoder available')
}

export function decodeJwt(token?: string | null): JwtClaims | null {
  if (!token) {
    return null
  }

  const parts = token.split('.')
  if (parts.length < 2) {
    return null
  }

  try {
    const payload = base64UrlDecode(parts[1])
    const parsed = JSON.parse(payload) as unknown
    if (parsed && typeof parsed === 'object') {
      return parsed as JwtClaims
    }
    return null
  } catch {
    return null
  }
}

export function getSessionIdFromToken(token?: string | null): string | null {
  const claims = decodeJwt(token)
  if (!claims) {
    return null
  }

  const candidates = [
    typeof claims.sid === 'string' ? claims.sid : null,
    typeof claims.session_id === 'string' ? claims.session_id : null,
    typeof claims.jti === 'string' ? claims.jti : null,
  ]

  for (const value of candidates) {
    if (value && value.trim().length > 0) {
      return value
    }
  }

  return null
}
