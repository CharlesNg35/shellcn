import type { ApiResponse } from '@/types/api'
import type { ActiveSessionParticipant, SessionParticipantsSummary } from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const BASE_ENDPOINT = '/active-sessions'

export interface AddSessionParticipantPayload {
  user_id: string
  role?: string
  access_mode?: 'read' | 'write'
  consented_to_recording?: boolean
}

export async function fetchSessionParticipants(
  sessionId: string
): Promise<SessionParticipantsSummary> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  const response = await apiClient.get<ApiResponse<SessionParticipantsSummary>>(
    `${BASE_ENDPOINT}/${encodeURIComponent(sessionId)}/participants`
  )
  return unwrapResponse(response)
}

export async function addSessionParticipant(
  sessionId: string,
  payload: AddSessionParticipantPayload
): Promise<ActiveSessionParticipant> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  const response = await apiClient.post<ApiResponse<ActiveSessionParticipant>>(
    `${BASE_ENDPOINT}/${encodeURIComponent(sessionId)}/participants`,
    payload
  )
  return unwrapResponse(response)
}

export async function removeSessionParticipant(sessionId: string, userId: string): Promise<void> {
  if (!sessionId || !userId) {
    throw new Error('sessionId and userId are required')
  }
  await apiClient.delete<ApiResponse<Record<string, unknown>>>(
    `${BASE_ENDPOINT}/${encodeURIComponent(sessionId)}/participants/${encodeURIComponent(userId)}`
  )
}

export async function grantSessionParticipantWrite(
  sessionId: string,
  userId: string
): Promise<ActiveSessionParticipant> {
  if (!sessionId || !userId) {
    throw new Error('sessionId and userId are required')
  }
  const response = await apiClient.post<ApiResponse<ActiveSessionParticipant>>(
    `${BASE_ENDPOINT}/${encodeURIComponent(sessionId)}/participants/${encodeURIComponent(userId)}/write`
  )
  return unwrapResponse(response)
}

export interface RelinquishWriteResult {
  participant: ActiveSessionParticipant
  write_holder?: ActiveSessionParticipant | null
}

export async function relinquishSessionParticipantWrite(
  sessionId: string,
  userId: string
): Promise<RelinquishWriteResult> {
  if (!sessionId || !userId) {
    throw new Error('sessionId and userId are required')
  }
  const response = await apiClient.delete<ApiResponse<RelinquishWriteResult>>(
    `${BASE_ENDPOINT}/${encodeURIComponent(sessionId)}/participants/${encodeURIComponent(userId)}/write`
  )
  return unwrapResponse(response)
}
