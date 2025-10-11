import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/api/auth', () => ({
  login: vi.fn(),
  verifyMfa: vi.fn(),
  fetchCurrentUser: vi.fn(),
  fetchProviders: vi.fn(),
  fetchSetupStatus: vi.fn().mockResolvedValue({ status: 'complete', message: '' }),
  initializeSetup: vi.fn(),
  logout: vi.fn(),
  requestPasswordReset: vi.fn(),
  confirmPasswordReset: vi.fn(),
}))

import { useAuthStore } from '@/store/auth-store'
import { clearTokens, getTokens } from '@/lib/api/token-storage'
import type { AuthTokens, AuthUser } from '@/types/auth'
import {
  login as loginApi,
  verifyMfa as verifyMfaApi,
  fetchCurrentUser as fetchCurrentUserApi,
  logout as logoutApi,
} from '@/lib/api/auth'

const mockedLogin = vi.mocked(loginApi)
const mockedVerifyMfa = vi.mocked(verifyMfaApi)
const mockedFetchCurrentUser = vi.mocked(fetchCurrentUserApi)
const mockedLogout = vi.mocked(logoutApi)

const demoTokens: AuthTokens = {
  accessToken: 'access-token',
  refreshToken: 'refresh-token',
  expiresAt: Date.now() + 15 * 60 * 1000,
  expiresIn: 900,
}

const demoUser: AuthUser = {
  id: 'usr_123',
  username: 'admin',
  email: 'admin@example.com',
  is_root: true,
  is_active: true,
  first_name: 'Ada',
  last_name: 'Admin',
}

function resetAuthStore() {
  useAuthStore.setState({
    status: 'unauthenticated',
    initialized: false,
    tokens: null,
    user: null,
    providers: [],
    mfaChallenge: undefined,
    error: null,
  })
}

describe('auth store', () => {
  beforeEach(() => {
    resetAuthStore()
    clearTokens()
    vi.clearAllMocks()
    window.localStorage.clear()
  })

  afterEach(() => {
    resetAuthStore()
    clearTokens()
    vi.clearAllMocks()
    window.localStorage.clear()
  })

  it('initializes to unauthenticated state when no tokens exist', async () => {
    await useAuthStore.getState().initialize()

    const state = useAuthStore.getState()
    expect(state.status).toBe('unauthenticated')
    expect(state.initialized).toBe(true)
    expect(state.tokens).toBeNull()
    expect(state.user).toBeNull()
  })

  it('logs in successfully and stores tokens and user', async () => {
    mockedLogin.mockResolvedValueOnce({
      tokens: demoTokens,
      user: demoUser,
    })

    await useAuthStore.getState().login({ identifier: 'admin', password: 'secret' })

    const state = useAuthStore.getState()
    expect(state.status).toBe('authenticated')
    expect(state.user).toEqual(demoUser)

    const storedTokens = getTokens()
    expect(storedTokens?.accessToken).toBe(demoTokens.accessToken)
    expect(storedTokens?.refreshToken).toBe(demoTokens.refreshToken)
  })

  it('handles MFA-required login', async () => {
    mockedLogin.mockResolvedValueOnce({
      mfaRequired: true,
      challenge: {
        challenge_id: 'chal_123',
        method: 'totp',
      },
    })

    const result = await useAuthStore.getState().login({ identifier: 'admin', password: 'secret' })

    const state = useAuthStore.getState()
    expect(result.mfaRequired).toBe(true)
    expect(state.status).toBe('mfa_required')
    expect(state.mfaChallenge?.challenge_id).toBe('chal_123')
    expect(getTokens()).toBeNull()
  })

  it('verifies MFA and restores authenticated session', async () => {
    useAuthStore.setState({
      status: 'mfa_required',
      initialized: false,
      tokens: null,
      user: null,
      providers: [],
      mfaChallenge: { challenge_id: 'chal_456', method: 'totp' },
      error: null,
    })

    mockedVerifyMfa.mockResolvedValueOnce(demoTokens)
    mockedFetchCurrentUser.mockResolvedValueOnce(demoUser)

    await useAuthStore.getState().verifyMfa({ challenge_id: 'chal_456', mfa_token: '123456' })

    const state = useAuthStore.getState()
    expect(state.status).toBe('authenticated')
    expect(state.user).toEqual(demoUser)
    expect(state.mfaChallenge).toBeUndefined()
    expect(getTokens()?.accessToken).toBe(demoTokens.accessToken)
  })

  it('clears state on logout', async () => {
    useAuthStore.setState({
      status: 'authenticated',
      initialized: true,
      tokens: demoTokens,
      user: demoUser,
      providers: [],
      mfaChallenge: undefined,
      error: null,
    })

    mockedLogout.mockResolvedValueOnce()

    await useAuthStore.getState().logout()

    const state = useAuthStore.getState()
    expect(state.status).toBe('unauthenticated')
    expect(state.user).toBeNull()
    expect(getTokens()).toBeNull()
  })
})
