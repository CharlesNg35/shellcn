import { create } from 'zustand'
import { toApiError } from '@/lib/api/http'
import {
  confirmPasswordReset as confirmPasswordResetApi,
  fetchCurrentUser,
  fetchProviders as fetchProvidersApi,
  fetchSetupStatus as fetchSetupStatusApi,
  initializeSetup as initializeSetupApi,
  login as apiLogin,
  logout as apiLogout,
  requestPasswordReset as requestPasswordResetApi,
  verifyMfa as apiVerifyMfa,
} from '@/lib/api/auth'
import { clearTokens, getTokens, setTokens, subscribeTokens } from '@/lib/api/token-storage'
import type {
  AuthProviderMetadata,
  AuthStatus,
  AuthTokens,
  AuthUser,
  LoginCredentials,
  LoginResult,
  MfaChallenge,
  PasswordResetConfirmPayload,
  PasswordResetRequestPayload,
  PasswordResetResponse,
  SetupInitializePayload,
  SetupInitializeResponse,
  SetupStatusPayload,
  VerifyMfaPayload,
} from '@/types/auth'

type AsyncResult<T> = Promise<T>

interface AuthState {
  status: AuthStatus
  initialized: boolean
  tokens: AuthTokens | null
  user: AuthUser | null
  providers: AuthProviderMetadata[]
  mfaChallenge?: MfaChallenge
  error: string | null
  errorCode: string | null
  setupStatus: SetupStatusPayload | null
  isSetupStatusLoading: boolean
}

interface AuthActions {
  initialize: () => AsyncResult<void>
  login: (credentials: LoginCredentials) => AsyncResult<LoginResult>
  verifyMfa: (payload: VerifyMfaPayload) => AsyncResult<void>
  refreshUser: () => AsyncResult<AuthUser | null>
  loadProviders: () => AsyncResult<AuthProviderMetadata[]>
  fetchSetupStatus: (options?: { force?: boolean }) => AsyncResult<SetupStatusPayload>
  completeSetup: (payload: SetupInitializePayload) => AsyncResult<SetupInitializeResponse>
  requestPasswordReset: (payload: PasswordResetRequestPayload) => AsyncResult<PasswordResetResponse>
  confirmPasswordReset: (payload: PasswordResetConfirmPayload) => AsyncResult<PasswordResetResponse>
  logout: () => AsyncResult<void>
  clearError: () => void
  setUser: (user: AuthUser | null) => void
}

export type AuthStore = AuthState & AuthActions

const tokensFromStorage = getTokens()

export const useAuthStore = create<AuthStore>((set, get) => {
  let cachedSetupStatus: SetupStatusPayload | null = null
  let setupStatusPromise: Promise<SetupStatusPayload> | null = null

  if (typeof window !== 'undefined') {
    subscribeTokens((tokens) => {
      set((state) => ({
        tokens,
        status: tokens ? (state.user ? 'authenticated' : 'loading') : 'unauthenticated',
        user: tokens ? state.user : null,
        mfaChallenge: tokens ? state.mfaChallenge : undefined,
      }))
    })
  }

  return {
    status: tokensFromStorage ? 'loading' : 'unauthenticated',
    initialized: false,
    tokens: tokensFromStorage,
    user: null,
    providers: [],
    mfaChallenge: undefined,
    error: null,
    errorCode: null,
    setupStatus: null,
    isSetupStatusLoading: false,

    initialize: async () => {
      const { tokens } = get()

      if (!tokens) {
        set({ status: 'unauthenticated', initialized: true, error: null, errorCode: null })
        return
      }

      set((state) => ({
        status: state.user ? 'authenticated' : 'loading',
        initialized: false,
      }))

      try {
        const user = await fetchCurrentUser()
        set({
          user,
          status: 'authenticated',
          initialized: true,
          error: null,
          errorCode: null,
        })
      } catch {
        clearTokens()
        set({
          tokens: null,
          status: 'unauthenticated',
          initialized: true,
          user: null,
          error: null,
          errorCode: null,
        })
      }
    },

    login: async (credentials) => {
      set({ status: 'loading', error: null, errorCode: null })

      try {
        const result = await apiLogin(credentials)

        if (result.mfaRequired) {
          set({
            status: 'mfa_required',
            mfaChallenge: result.challenge,
            error: null,
            errorCode: null,
          })

          return result
        }

        if (result.tokens) {
          setTokens(result.tokens)
        }

        if (!result.user) {
          await get().refreshUser()
        } else {
          set({ user: result.user })
        }

        set({
          status: 'authenticated',
          initialized: true,
          mfaChallenge: undefined,
          error: null,
          errorCode: null,
        })

        return result
      } catch (error) {
        const apiError = toApiError(error)
        set({
          status: 'unauthenticated',
          error: apiError.message,
          errorCode: apiError.code ?? null,
          mfaChallenge: undefined,
        })
        throw apiError
      }
    },

    verifyMfa: async (payload) => {
      set({ status: 'loading', error: null, errorCode: null })

      try {
        const tokens = await apiVerifyMfa(payload)

        if (tokens) {
          setTokens(tokens)
        }

        await get().refreshUser()

        set({
          status: 'authenticated',
          mfaChallenge: undefined,
          error: null,
          errorCode: null,
        })
      } catch (error) {
        const apiError = toApiError(error)
        set({
          status: 'unauthenticated',
          mfaChallenge: undefined,
          error: apiError.message,
          errorCode: apiError.code ?? null,
        })
        throw apiError
      }
    },

    refreshUser: async () => {
      try {
        const user = await fetchCurrentUser()
        set({
          user,
          status: 'authenticated',
          error: null,
          errorCode: null,
          initialized: true,
        })
        return user
      } catch (error) {
        clearTokens()
        const apiError = toApiError(error)
        set({
          tokens: null,
          user: null,
          status: 'unauthenticated',
          error: apiError.message,
          errorCode: apiError.code ?? null,
        })
        return null
      }
    },

    loadProviders: async () => {
      try {
        const providers = await fetchProvidersApi()
        set({ providers })
        return providers
      } catch (error) {
        console.error('Failed to load auth providers', error)
        throw error
      }
    },

    fetchSetupStatus: async (options) => {
      const force = options?.force ?? false

      if (!force) {
        const { setupStatus } = get()
        if (setupStatus) {
          return setupStatus
        }
      }

      if (!force && cachedSetupStatus) {
        set({ setupStatus: cachedSetupStatus })
        return cachedSetupStatus
      }

      if (!force && setupStatusPromise) {
        return setupStatusPromise
      }

      const request = fetchSetupStatusApi()
        .then((status) => {
          cachedSetupStatus = status
          setupStatusPromise = null
          set({ setupStatus: status, isSetupStatusLoading: false })
          return status
        })
        .catch((error) => {
          setupStatusPromise = null
          set({ isSetupStatusLoading: false })
          throw error
        })

      setupStatusPromise = request
      set({ isSetupStatusLoading: true })
      return request
    },

    completeSetup: async (payload) => {
      const result = await initializeSetupApi(payload)
      cachedSetupStatus = { status: 'complete', message: result.message }
      set({ setupStatus: cachedSetupStatus, isSetupStatusLoading: false })
      return result
    },

    requestPasswordReset: async (payload) => {
      const result = await requestPasswordResetApi(payload)
      return result
    },

    confirmPasswordReset: async (payload) => {
      const result = await confirmPasswordResetApi(payload)
      return result
    },

    logout: async () => {
      try {
        await apiLogout()
      } finally {
        clearTokens()
        cachedSetupStatus = null
        setupStatusPromise = null
        set({
          tokens: null,
          user: null,
          status: 'unauthenticated',
          initialized: true,
          mfaChallenge: undefined,
          error: null,
          errorCode: null,
          setupStatus: null,
          isSetupStatusLoading: false,
        })
      }
    },

    clearError: () => set({ error: null, errorCode: null }),

    setUser: (user) => set({ user }),
  }
})
