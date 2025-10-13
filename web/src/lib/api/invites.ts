import { apiClient } from './client'
import { unwrapResponse } from './http'
import type { ApiResponse } from '@/types/api'
import type { InviteCreatePayload, InviteCreateResponse, InviteRecord } from '@/types/invites'

const INVITES_ENDPOINT = '/invites'
const AUTH_INVITE_ACCEPT_ENDPOINT = '/auth/invite/redeem'

export async function createInvite(payload: InviteCreatePayload): Promise<InviteCreateResponse> {
  const response = await apiClient.post<ApiResponse<InviteCreateResponse>>(
    INVITES_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

export async function fetchInvites(params?: {
  status?: string
  search?: string
}): Promise<InviteRecord[]> {
  const response = await apiClient.get<ApiResponse<{ invites: InviteRecord[] }>>(INVITES_ENDPOINT, {
    params,
  })
  const data = unwrapResponse(response)
  return data.invites
}

export async function deleteInvite(inviteId: string): Promise<void> {
  const response = await apiClient.delete<ApiResponse<{ deleted: boolean }>>(
    `${INVITES_ENDPOINT}/${inviteId}`
  )
  unwrapResponse(response)
}

export interface RedeemInvitePayload {
  token: string
  username: string
  password: string
  first_name?: string
  last_name?: string
}

export async function redeemInvite(payload: RedeemInvitePayload): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(AUTH_INVITE_ACCEPT_ENDPOINT, payload)
  unwrapResponse(response)
}

export const invitesApi = {
  create: createInvite,
  list: fetchInvites,
  delete: deleteInvite,
  redeem: redeemInvite,
}
