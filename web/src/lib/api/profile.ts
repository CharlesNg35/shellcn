import type { ApiResponse } from '@/types/api'
import type { AuthUser } from '@/types/auth'
import type { UserPreferences } from '@/types/preferences'
import type {
  MfaSetupResponse,
  PasswordChangePayload,
  ProfileUpdatePayload,
  TotpCodePayload,
} from '@/types/profile'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import { transformAuthUser } from './transformers'

const PROFILE_ENDPOINT = '/profile'
const PROFILE_PASSWORD_ENDPOINT = '/profile/password'
const PROFILE_MFA_SETUP_ENDPOINT = '/profile/mfa/setup'
const PROFILE_MFA_ENABLE_ENDPOINT = '/profile/mfa/enable'
const PROFILE_MFA_DISABLE_ENDPOINT = '/profile/mfa/disable'
const PROFILE_EMAIL_RESEND_ENDPOINT = '/auth/email/resend'
const PROFILE_PREFERENCES_ENDPOINT = '/profile/preferences'

export async function updateProfile(payload: ProfileUpdatePayload): Promise<AuthUser> {
  const response = await apiClient.patch<ApiResponse<AuthUser>>(PROFILE_ENDPOINT, payload)
  const result = unwrapResponse(response)
  return transformAuthUser(result) ?? result
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

export async function resendEmailVerification(): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(PROFILE_EMAIL_RESEND_ENDPOINT)
}

function normalisePreferences(payload?: Partial<UserPreferences>): UserPreferences {
  const ssh = payload?.ssh ?? ({} as Partial<UserPreferences['ssh']>)
  const terminal = ssh.terminal ?? ({} as Partial<UserPreferences['ssh']['terminal']>)
  const sftp = ssh.sftp ?? ({} as Partial<UserPreferences['ssh']['sftp']>)

  return {
    ssh: {
      terminal: {
        font_family: terminal.font_family?.trim() || 'Fira Code',
        cursor_style: (terminal.cursor_style ??
          'block') as UserPreferences['ssh']['terminal']['cursor_style'],
        copy_on_select: terminal.copy_on_select !== false,
        font_size:
          typeof terminal.font_size === 'number' && terminal.font_size >= 8
            ? terminal.font_size
            : 14,
        scrollback_limit:
          typeof terminal.scrollback_limit === 'number' && terminal.scrollback_limit >= 200
            ? terminal.scrollback_limit
            : 1000,
      },
      sftp: {
        show_hidden_files: Boolean(sftp.show_hidden_files),
        auto_open_queue: sftp.auto_open_queue !== false,
        confirm_before_overwrite: sftp.confirm_before_overwrite !== false,
      },
    },
  }
}

export async function fetchUserPreferences(): Promise<UserPreferences> {
  const response = await apiClient.get<ApiResponse<UserPreferences>>(PROFILE_PREFERENCES_ENDPOINT)
  const data = unwrapResponse(response)
  return normalisePreferences(data)
}

export async function updateUserPreferences(payload: UserPreferences): Promise<UserPreferences> {
  const response = await apiClient.put<ApiResponse<UserPreferences>>(
    PROFILE_PREFERENCES_ENDPOINT,
    payload
  )
  const data = unwrapResponse(response)
  return normalisePreferences(data)
}
