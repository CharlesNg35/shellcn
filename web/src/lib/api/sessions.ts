import type { ApiResponse } from '@/types/api'
import type { SessionPayload } from '@/types/sessions'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const MY_SESSIONS_ENDPOINT = '/sessions/me'
const REVOKE_SESSION_ENDPOINT = '/sessions/revoke'
const REVOKE_ALL_ENDPOINT = '/sessions/revoke_all'

export async function fetchMySessions(): Promise<SessionPayload[]> {
  const response = await apiClient.get<ApiResponse<SessionPayload[]>>(MY_SESSIONS_ENDPOINT)
  return unwrapResponse(response)
}

export async function revokeSession(sessionId: string): Promise<void> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }

  await apiClient.post<ApiResponse<Record<string, unknown>>>(
    `${REVOKE_SESSION_ENDPOINT}/${encodeURIComponent(sessionId)}`
  )
}

export async function revokeAllOtherSessions(): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(REVOKE_ALL_ENDPOINT)
}

export const sessionsApi = {
  listMine: fetchMySessions,
  revoke: revokeSession,
  revokeAll: revokeAllOtherSessions,
}
