import type { ApiResponse } from '@/types/api'
import type {
  AuthProviderMetadata,
  AuthTokens,
  AuthUser,
  LoginCredentials,
  LoginResponsePayload,
  LoginResult,
  MfaChallenge,
  PasswordResetConfirmPayload,
  PasswordResetRequestPayload,
  PasswordResetResponse,
  RegistrationPayload,
  RegistrationResponse,
  SetupInitializePayload,
  SetupInitializeResponse,
  SetupStatusPayload,
  VerifyMfaPayload,
} from '@/types/auth'
import { apiClient } from './client'
import { ApiError, unwrapResponse } from './http'
import { toAuthTokens, transformAuthUser } from './transformers'

type LoginApiPayload = LoginResponsePayload & {
  mfa_required?: boolean
  challenge?: MfaChallengeLike
  challenge_id?: string
}

type MfaChallengeLike = Partial<MfaChallenge> & {
  id?: string
  challenge_id?: string
  method?: string
  methods?: string[]
  expires_at?: string
  details?: Record<string, unknown>
}

const AUTH_LOGIN_ENDPOINT = '/auth/login'
const AUTH_LOGOUT_ENDPOINT = '/auth/logout'
const AUTH_ME_ENDPOINT = '/auth/me'
const AUTH_MFA_VERIFY_ENDPOINT = '/auth/mfa/verify'
const AUTH_PROVIDERS_ENDPOINT = '/auth/providers'
const AUTH_PASSWORD_RESET_REQUEST_ENDPOINT = '/auth/password-reset/request'
const AUTH_PASSWORD_RESET_CONFIRM_ENDPOINT = '/auth/password-reset/confirm'
const AUTH_REGISTER_ENDPOINT = '/auth/register'
const SETUP_STATUS_ENDPOINT = '/setup/status'
const SETUP_INITIALIZE_ENDPOINT = '/setup/initialize'

export async function login(credentials: LoginCredentials): Promise<LoginResult> {
  try {
    const response = await apiClient.post<ApiResponse<LoginApiPayload>>(
      AUTH_LOGIN_ENDPOINT,
      credentials
    )
    const data = unwrapResponse(response)

    if (data.mfa_required || data.challenge) {
      return {
        mfaRequired: true,
        challenge: normalizeChallenge(
          data.challenge ?? {
            challenge_id: data.challenge_id,
            method: 'totp',
          }
        ),
      }
    }

    const tokens = toAuthTokens(data)

    return {
      tokens: tokens ?? undefined,
      user: transformAuthUser(data.user) ?? undefined,
    }
  } catch (error) {
    if (error instanceof ApiError && error.code === 'auth.mfa_required') {
      return {
        mfaRequired: true,
        challenge: normalizeChallenge(
          (error.details?.challenge as unknown) ?? (error.details as unknown)
        ),
      }
    }

    throw error
  }
}

export async function verifyMfa(payload: VerifyMfaPayload): Promise<AuthTokens | null> {
  const response = await apiClient.post<ApiResponse<LoginResponsePayload>>(
    AUTH_MFA_VERIFY_ENDPOINT,
    payload
  )
  const data = unwrapResponse(response)

  const tokens = toAuthTokens(data)

  return tokens
}

export async function registerLocal(payload: RegistrationPayload): Promise<RegistrationResponse> {
  const response = await apiClient.post<ApiResponse<RegistrationResponse>>(
    AUTH_REGISTER_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

export async function fetchCurrentUser(): Promise<AuthUser> {
  const response = await apiClient.get<ApiResponse<AuthUser>>(AUTH_ME_ENDPOINT)
  const result = unwrapResponse(response)
  const normalized = transformAuthUser(result)
  return normalized ?? result
}

export async function logout(): Promise<void> {
  const response = await apiClient.post<ApiResponse<{ message?: string }>>(AUTH_LOGOUT_ENDPOINT)
  unwrapResponse(response)
}

export async function fetchProviders(): Promise<AuthProviderMetadata[]> {
  const response = await apiClient.get<ApiResponse<AuthProviderMetadata[]>>(AUTH_PROVIDERS_ENDPOINT)
  return unwrapResponse(response)
}

export async function requestPasswordReset(
  payload: PasswordResetRequestPayload
): Promise<PasswordResetResponse> {
  const response = await apiClient.post<ApiResponse<PasswordResetResponse>>(
    AUTH_PASSWORD_RESET_REQUEST_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

export async function confirmPasswordReset(
  payload: PasswordResetConfirmPayload
): Promise<PasswordResetResponse> {
  const response = await apiClient.post<ApiResponse<PasswordResetResponse>>(
    AUTH_PASSWORD_RESET_CONFIRM_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

export async function fetchSetupStatus(): Promise<SetupStatusPayload> {
  const response = await apiClient.get<ApiResponse<SetupStatusPayload>>(SETUP_STATUS_ENDPOINT)
  return unwrapResponse(response)
}

export async function initializeSetup(
  payload: SetupInitializePayload
): Promise<SetupInitializeResponse> {
  const response = await apiClient.post<ApiResponse<SetupInitializeResponse>>(
    SETUP_INITIALIZE_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

function normalizeChallenge(source?: unknown): MfaChallenge | undefined {
  if (!source || typeof source !== 'object') {
    return undefined
  }

  const challenge = source as MfaChallengeLike
  const challengeId = challenge.challenge_id ?? challenge.id

  if (!challengeId) {
    return undefined
  }

  return {
    challenge_id: challengeId,
    method: challenge.method ?? 'totp',
    methods: challenge.methods,
    expires_at: challenge.expires_at,
    details: challenge.details,
  }
}
