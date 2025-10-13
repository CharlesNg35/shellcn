import type { InviteCreateResponse } from '@/types/invites'

/**
 * Returns an absolute invite link using the response payload.
 * Falls back to constructing the link from the invite token if necessary.
 */
export function buildInviteLink(result: InviteCreateResponse): string {
  const fallbackPath = `/invite/accept?token=${encodeURIComponent(result.token)}`
  const raw = result.link?.trim() || fallbackPath

  if (raw.startsWith('http://') || raw.startsWith('https://')) {
    return raw
  }

  if (typeof window !== 'undefined' && window.location?.origin) {
    return `${window.location.origin}${raw.startsWith('/') ? raw : `/${raw}`}`
  }

  return raw
}
