import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useShallow } from 'zustand/react/shallow'
import { toApiError } from '@/lib/api/http'
import {
  changePassword as changePasswordRequest,
  disableMfa as disableMfaRequest,
  enableMfa as enableMfaRequest,
  setupMfa as setupMfaRequest,
  updateProfile as updateProfileRequest,
} from '@/lib/api/profile'
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
