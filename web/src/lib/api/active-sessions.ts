import type { ApiResponse } from '@/types/api'
import type { ActiveConnectionSession } from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const ACTIVE_SESSIONS_ENDPOINT = '/active-sessions'

export interface LaunchSessionPayload {
  connection_id: string
  protocol_id?: string
  fields_override?: Record<string, unknown>
}

export interface LaunchSessionTunnel {
  url: string
  token: string
  expires_at?: string
}

export interface LaunchSessionDescriptor {
  id: string
  protocol_id?: string
  default_route?: string
  [key: string]: unknown
}

export interface LaunchSessionResponse {
  session: ActiveConnectionSession
  tunnel?: LaunchSessionTunnel
  descriptor?: LaunchSessionDescriptor
}

export async function launchActiveSession(
  payload: LaunchSessionPayload
): Promise<LaunchSessionResponse> {
  const response = await apiClient.post<ApiResponse<LaunchSessionResponse>>(
    ACTIVE_SESSIONS_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}
