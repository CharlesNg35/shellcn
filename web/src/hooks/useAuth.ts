import { useCallback, useEffect, useMemo } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useAuthStore, type AuthStore } from '@/store/auth-store'
import type {
  LoginCredentials,
  PasswordResetConfirmPayload,
  PasswordResetRequestPayload,
  SetupInitializePayload,
  VerifyMfaPayload,
} from '@/types/auth'

interface UseAuthOptions {
  autoInitialize?: boolean
}

const selector = (state: AuthStore) => ({
  status: state.status,
  initialized: state.initialized,
  tokens: state.tokens,
  user: state.user,
  providers: state.providers,
  mfaChallenge: state.mfaChallenge,
  error: state.error,
  initialize: state.initialize,
  login: state.login,
  verifyMfa: state.verifyMfa,
  refreshUser: state.refreshUser,
  loadProviders: state.loadProviders,
  fetchSetupStatus: state.fetchSetupStatus,
  completeSetup: state.completeSetup,
  logout: state.logout,
  clearError: state.clearError,
  requestPasswordReset: state.requestPasswordReset,
  confirmPasswordReset: state.confirmPasswordReset,
  setupStatus: state.setupStatus,
  isSetupStatusLoading: state.isSetupStatusLoading,
  errorCode: state.errorCode,
})

export function useAuth(options: UseAuthOptions = {}) {
  const selected = useAuthStore(useShallow(selector))
  const {
    status,
    initialized,
    tokens,
    user,
    providers,
    mfaChallenge,
    setupStatus,
    isSetupStatusLoading,
    error,
    errorCode,
    initialize,
    login,
    verifyMfa,
    refreshUser,
    loadProviders,
    fetchSetupStatus,
    completeSetup,
    logout,
    clearError,
    requestPasswordReset,
    confirmPasswordReset,
  } = selected

  const { autoInitialize = true } = options

  useEffect(() => {
    if (autoInitialize && !initialized) {
      void initialize()
    }
  }, [autoInitialize, initialized, initialize])

  const loginWithCredentials = useCallback(
    (credentials: LoginCredentials) => login(credentials),
    [login]
  )

  const verifyMfaCode = useCallback((payload: VerifyMfaPayload) => verifyMfa(payload), [verifyMfa])

  const requestReset = useCallback(
    (payload: PasswordResetRequestPayload) => requestPasswordReset(payload),
    [requestPasswordReset]
  )

  const confirmReset = useCallback(
    (payload: PasswordResetConfirmPayload) => confirmPasswordReset(payload),
    [confirmPasswordReset]
  )

  const completeInitialSetup = useCallback(
    (payload: SetupInitializePayload) => completeSetup(payload),
    [completeSetup]
  )

  const getSetupStatus = useCallback(() => fetchSetupStatus(), [fetchSetupStatus])

  return useMemo(
    () => ({
      status,
      initialized,
      tokens,
      user,
      providers,
      mfaChallenge,
      error,
      errorCode,
      initialize,
      login: loginWithCredentials,
      verifyMfa: verifyMfaCode,
      refreshUser,
      loadProviders,
      fetchSetupStatus: getSetupStatus,
      completeSetup: completeInitialSetup,
      logout,
      clearError,
      requestPasswordReset: requestReset,
      confirmPasswordReset: confirmReset,
      setupStatus,
      isSetupStatusLoading,
      isAuthenticated: status === 'authenticated' && Boolean(user),
      isLoading: status === 'loading',
      isMfaRequired: status === 'mfa_required',
    }),
    [
      status,
      initialized,
      tokens,
      user,
      providers,
      mfaChallenge,
      error,
      errorCode,
      initialize,
      loginWithCredentials,
      verifyMfaCode,
      refreshUser,
      loadProviders,
      getSetupStatus,
      completeInitialSetup,
      logout,
      clearError,
      requestReset,
      confirmReset,
      setupStatus,
      isSetupStatusLoading,
    ]
  )
}
