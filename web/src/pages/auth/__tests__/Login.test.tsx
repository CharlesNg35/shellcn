import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { render, screen } from '@testing-library/react'

vi.mock('@/hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '@/hooks/useAuth'
import { Login } from '@/pages/auth/Login'

const mockedUseAuth = vi.mocked(useAuth)

const baseAuth = {
  login: vi.fn(),
  isLoading: false,
  error: null as string | null,
  clearError: vi.fn(),
  isMfaRequired: false,
  fetchSetupStatus: vi.fn().mockResolvedValue({ status: 'complete', message: '' }),
  status: 'unauthenticated' as const,
  providers: [] as {
    type: string
    name: string
    enabled: boolean
    allow_registration?: boolean
    allow_password_reset?: boolean
    flow?: string
  }[],
  loadProviders: vi.fn(),
  setupStatus: { status: 'complete', message: '' },
  mfaChallenge: undefined,
  initialized: true,
  tokens: null,
  user: null,
  refreshUser: vi.fn(),
  verifyMfa: vi.fn(),
  completeSetup: vi.fn(),
  logout: vi.fn(),
  requestPasswordReset: vi.fn(),
  confirmPasswordReset: vi.fn(),
  setupStatusLoading: false,
  isSetupStatusLoading: false,
  errorCode: null as string | null,
  providersLoaded: true,
}

afterEach(() => {
  vi.clearAllMocks()
})

function renderLogin(initialEntry = '/login') {
  render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<div>Register</div>} />
        <Route path="/setup" element={<div>Setup</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('Login page', () => {
  it('shows registration link when local provider allows registration', () => {
    mockedUseAuth.mockReturnValue({
      ...baseAuth,
      providers: [
        {
          type: 'local',
          name: 'Local Authentication',
          enabled: true,
          allow_registration: true,
          allow_password_reset: true,
          flow: 'password',
        },
      ],
    })

    renderLogin()

    expect(screen.getByText('Create one')).toBeInTheDocument()
  })

  it('displays info message when registration succeeds', () => {
    mockedUseAuth.mockReturnValue({
      ...baseAuth,
    })

    renderLogin('/login?notice=register_success')

    expect(screen.getByText('Account created. You can now sign in.')).toBeInTheDocument()
  })
})
