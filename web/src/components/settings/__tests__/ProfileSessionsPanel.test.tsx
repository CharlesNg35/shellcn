import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'

vi.mock('@/hooks/useProfileSettings', () => ({
  useProfileSessions: vi.fn(),
}))

import { ProfileSessionsPanel } from '@/components/settings/ProfileSessionsPanel'
import { useProfileSessions } from '@/hooks/useProfileSettings'
import type { UseProfileSessionsResult } from '@/hooks/useProfileSettings'

const revokeSessionMock = vi.fn()
const revokeOtherSessionsMock = vi.fn()

const mockedUseProfileSessions = vi.mocked(useProfileSessions)

describe('ProfileSessionsPanel', () => {
  beforeEach(() => {
    revokeSessionMock.mockReset()
    revokeOtherSessionsMock.mockReset()

    const mockedResult: UseProfileSessionsResult = {
      sessions: [
        {
          id: 'sess-1',
          user_id: 'user-1',
          ip_address: '192.168.1.10',
          user_agent: 'Chrome',
          device_name: 'Chrome',
          expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
          last_used_at: new Date().toISOString(),
          created_at: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
          updated_at: null,
          revoked_at: null,
          status: 'active',
          is_active: true,
          is_current: true,
        },
        {
          id: 'sess-2',
          user_id: 'user-1',
          ip_address: '10.0.0.2',
          user_agent: 'Firefox',
          device_name: 'Firefox',
          expires_at: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
          last_used_at: new Date(Date.now() - 60 * 1000).toISOString(),
          created_at: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
          updated_at: null,
          revoked_at: null,
          status: 'active',
          is_active: true,
          is_current: false,
        },
      ],
      currentSessionId: 'sess-1',
      stats: {
        total: 2,
        active: 2,
        otherActive: 1,
        revoked: 0,
        expired: 0,
      },
      query: {
        isError: false,
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: vi.fn(),
        data: [],
      } as unknown as UseProfileSessionsResult['query'],
      revokeSession: {
        mutate: revokeSessionMock,
        isPending: false,
        variables: undefined,
      } as unknown as UseProfileSessionsResult['revokeSession'],
      revokeOtherSessions: {
        mutate: revokeOtherSessionsMock,
        isPending: false,
      } as unknown as UseProfileSessionsResult['revokeOtherSessions'],
    }

    mockedUseProfileSessions.mockReturnValue(mockedResult)
  })

  it('renders sessions and disables revoke for current session', () => {
    render(<ProfileSessionsPanel />)

    const revokeButtons = screen.getAllByRole('button', { name: /^Revoke$/i })
    expect(revokeButtons).toHaveLength(2)
    const [currentButton, otherButton] = revokeButtons

    expect(currentButton).toBeDisabled()
    expect(otherButton).toBeEnabled()
  })

  it('triggers revoke handlers', () => {
    render(<ProfileSessionsPanel />)

    const otherButton = screen.getAllByRole('button', { name: /^Revoke$/i })[1]
    fireEvent.click(otherButton)
    expect(revokeSessionMock).toHaveBeenCalledWith('sess-2')

    const revokeOthersButton = screen.getByRole('button', { name: /Revoke Other Sessions/i })
    fireEvent.click(revokeOthersButton)
    expect(revokeOtherSessionsMock).toHaveBeenCalled()
  })
})
