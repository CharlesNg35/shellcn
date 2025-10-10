import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { vi } from 'vitest'
import { LocalSettingsForm } from '@/components/auth-providers/LocalSettingsForm'
import type { AuthProviderRecord } from '@/types/auth-providers'

const { mockUpdateLocal, mockToastSuccess, mockToastError } = vi.hoisted(() => ({
  mockUpdateLocal: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}))

vi.mock('@/lib/api/auth-providers', () => ({
  authProvidersApi: {
    updateLocalSettings: mockUpdateLocal,
  },
}))

vi.mock('@/lib/utils/toast', () => ({
  toast: {
    success: mockToastSuccess,
    error: mockToastError,
  },
}))

function renderWithClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('LocalSettingsForm', () => {
  beforeEach(() => {
    mockUpdateLocal.mockResolvedValue(undefined)
    mockUpdateLocal.mockClear()
    mockToastSuccess.mockClear()
    mockToastError.mockClear()
  })

  it('submits updated settings', async () => {
    const provider: AuthProviderRecord = {
      id: 'local',
      type: 'local',
      name: 'Local',
      enabled: true,
      allowRegistration: true,
      requireEmailVerification: true,
      allowPasswordReset: true,
    }

    renderWithClient(<LocalSettingsForm provider={provider} />)

    const checkboxes = screen.getAllByRole('checkbox')
    // Toggle self-registration off
    fireEvent.click(checkboxes[0])

    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => {
      expect(mockUpdateLocal).toHaveBeenCalledWith(
        expect.objectContaining({
          allowRegistration: false,
          requireEmailVerification: true,
          allowPasswordReset: true,
        })
      )
    })
  })
})
