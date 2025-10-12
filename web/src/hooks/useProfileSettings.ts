import { useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'
import { useShallow } from 'zustand/react/shallow'
import { toApiError, type ApiError } from '@/lib/api/http'
import {
  changePassword as changePasswordRequest,
  disableMfa as disableMfaRequest,
  enableMfa as enableMfaRequest,
  setupMfa as setupMfaRequest,
  updateProfile as updateProfileRequest,
} from '@/lib/api/profile'
import { sessionsApi } from '@/lib/api/sessions'
import { getSessionIdFromToken } from '@/lib/utils/jwt'
import { toast } from '@/lib/utils/toast'
import { CURRENT_USER_QUERY_KEY } from '@/hooks/useCurrentUser'
import type {
  MfaSetupResponse,
  PasswordChangePayload,
  ProfileUpdatePayload,
  TotpCodePayload,
} from '@/types/profile'
import type { AuthUser } from '@/types/auth'
import { useAuthStore } from '@/store/auth-store'
import type { SessionPayload, SessionStatus } from '@/types/sessions'

export function useProfileSettings() {
  const queryClient = useQueryClient()
  const { refreshUser, setUser } = useAuthStore(
    useShallow((state) => ({
      refreshUser: state.refreshUser,
      setUser: state.setUser,
    }))
  )

  const updateProfile = useMutation<AuthUser, unknown, ProfileUpdatePayload>({
    mutationFn: updateProfileRequest,
    onSuccess: async (user) => {
      setUser(user)
      await queryClient.invalidateQueries({ queryKey: CURRENT_USER_QUERY_KEY })
      toast.success('Profile updated')
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Unable to update profile', {
        description: apiError.message,
      })
    },
  })

  const changePassword = useMutation<void, unknown, PasswordChangePayload>({
    mutationFn: changePasswordRequest,
    onSuccess: () => {
      toast.success('Password updated successfully')
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Password update failed', {
        description: apiError.message,
      })
    },
  })

  const setupMfa = useMutation<MfaSetupResponse, unknown, void>({
    mutationFn: () => setupMfaRequest(),
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Unable to start MFA setup', {
        description: apiError.message,
      })
    },
  })

  const enableMfa = useMutation<void, unknown, TotpCodePayload>({
    mutationFn: enableMfaRequest,
    onSuccess: async () => {
      await refreshUser()
      await queryClient.invalidateQueries({ queryKey: CURRENT_USER_QUERY_KEY })
      toast.success('Multi-factor authentication enabled')
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to enable MFA', {
        description: apiError.message,
      })
    },
  })

  const disableMfa = useMutation<void, unknown, TotpCodePayload>({
    mutationFn: disableMfaRequest,
    onSuccess: async () => {
      await refreshUser()
      await queryClient.invalidateQueries({ queryKey: CURRENT_USER_QUERY_KEY })
      toast.success('Multi-factor authentication disabled')
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to disable MFA', {
        description: apiError.message,
      })
    },
  })

  return {
    updateProfile,
    changePassword,
    setupMfa,
    enableMfa,
    disableMfa,
  }
}

const PROFILE_SESSIONS_QUERY_KEY = ['profile', 'sessions'] as const

export interface ProfileSession extends SessionPayload {
  status: SessionStatus
  is_active: boolean
  is_current: boolean
}

export interface UseProfileSessionsResult {
  sessions: ProfileSession[]
  currentSessionId: string | null
  stats: {
    total: number
    active: number
    otherActive: number
    revoked: number
    expired: number
  }
  query: UseQueryResult<SessionPayload[], ApiError>
  revokeSession: UseMutationResult<void, ApiError, string, unknown>
  revokeOtherSessions: UseMutationResult<void, ApiError, void, unknown>
}

function deriveSessionStatus(session: SessionPayload): SessionStatus {
  if (session.revoked_at) {
    return 'revoked'
  }
  const expires = Date.parse(session.expires_at)
  if (!Number.isNaN(expires) && expires <= Date.now()) {
    return 'expired'
  }
  return 'active'
}

export function useProfileSessions(): UseProfileSessionsResult {
  const queryClient = useQueryClient()
  const { tokens } = useAuthStore(
    useShallow((state) => ({
      tokens: state.tokens,
    }))
  )

  const currentSessionId = useMemo(
    () => getSessionIdFromToken(tokens?.accessToken ?? null),
    [tokens?.accessToken]
  )

  const query = useQuery<SessionPayload[], ApiError>({
    queryKey: PROFILE_SESSIONS_QUERY_KEY,
    queryFn: sessionsApi.listMine,
    staleTime: 30_000,
    gcTime: 5 * 60_000,
  })

  const revokeSession = useMutation<void, ApiError, string>({
    mutationFn: async (sessionId: string) => {
      await sessionsApi.revoke(sessionId)
    },
    onSuccess: async (_, sessionId) => {
      await queryClient.invalidateQueries({ queryKey: PROFILE_SESSIONS_QUERY_KEY })
      toast.success('Session revoked', {
        description: 'The selected session has been terminated.',
      })
      return sessionId
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to revoke session', {
        description: apiError.message,
      })
      return apiError
    },
  })

  const revokeOtherSessions = useMutation<void, ApiError, void>({
    mutationFn: async () => {
      await sessionsApi.revokeAll()
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: PROFILE_SESSIONS_QUERY_KEY })
      toast.success('Sessions revoked', {
        description: 'All other sessions have been terminated.',
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to revoke sessions', {
        description: apiError.message,
      })
      return apiError
    },
  })

  const sessions = useMemo<ProfileSession[]>(() => {
    const data = query.data ?? []
    return data
      .map((session) => {
        const status = deriveSessionStatus(session)
        const isCurrent = session.id === currentSessionId
        return {
          ...session,
          status,
          is_active: status === 'active',
          is_current: isCurrent,
        }
      })
      .sort((a, b) => Date.parse(b.last_used_at) - Date.parse(a.last_used_at))
  }, [currentSessionId, query.data])

  const stats = useMemo(() => {
    const active = sessions.filter((session) => session.status === 'active').length
    const revoked = sessions.filter((session) => session.status === 'revoked').length
    const expired = sessions.filter((session) => session.status === 'expired').length
    const otherActive = sessions.filter(
      (session) => session.status === 'active' && !session.is_current
    ).length

    return {
      total: sessions.length,
      active,
      otherActive,
      revoked,
      expired,
    }
  }, [sessions])

  return {
    sessions,
    currentSessionId,
    stats,
    query,
    revokeSession,
    revokeOtherSessions,
  }
}
