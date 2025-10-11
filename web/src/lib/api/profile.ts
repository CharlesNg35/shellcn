import type { ApiResponse } from '@/types/api'
import type { AuthUser } from '@/types/auth'
import type {
  MfaSetupResponse,
  PasswordChangePayload,
  ProfileUpdatePayload,
  TotpCodePayload,
} from '@/types/profile'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const PROFILE_ENDPOINT = '/profile'
const PROFILE_PASSWORD_ENDPOINT = '/profile/password'
const PROFILE_MFA_SETUP_ENDPOINT = '/profile/mfa/setup'
const PROFILE_MFA_ENABLE_ENDPOINT = '/profile/mfa/enable'
const PROFILE_MFA_DISABLE_ENDPOINT = '/profile/mfa/disable'

export async function updateProfile(payload: ProfileUpdatePayload): Promise<AuthUser> {
  const response = await apiClient.patch<ApiResponse<AuthUser>>(PROFILE_ENDPOINT, payload)
  return unwrapResponse(response)
}

export async function changePassword(payload: PasswordChangePayload): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(PROFILE_PASSWORD_ENDPOINT, payload)
}

export async function setupMfa(): Promise<MfaSetupResponse> {
  const response = await apiClient.post<ApiResponse<MfaSetupResponse>>(PROFILE_MFA_SETUP_ENDPOINT)
  return unwrapResponse(response)
}

export async function enableMfa(payload: TotpCodePayload): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(PROFILE_MFA_ENABLE_ENDPOINT, payload)
}

export async function disableMfa(payload: TotpCodePayload): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(PROFILE_MFA_DISABLE_ENDPOINT, payload)
}
