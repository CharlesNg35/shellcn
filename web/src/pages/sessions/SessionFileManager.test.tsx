import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import SessionFileManager from '@/pages/sessions/SessionFileManager'

const mockUseActiveConnections = vi.fn()
const mockUseCurrentUser = vi.fn()
const mockSetOverride = vi.fn()
const mockClearOverride = vi.fn()

vi.mock('@/hooks/useActiveConnections', () => ({
  useActiveConnections: (...args: unknown[]) => mockUseActiveConnections(...args),
}))

vi.mock('@/hooks/useCurrentUser', () => ({
  useCurrentUser: () => mockUseCurrentUser(),
}))

const workspaceMock = vi.fn(() => <div data-testid="sftp-workspace-mock" />)

vi.mock('@/components/workspace/SftpWorkspace', () => ({
  SftpWorkspace: (...args: unknown[]) => workspaceMock(...args),
  default: (...args: unknown[]) => workspaceMock(...args),
}))

vi.mock('@/contexts/BreadcrumbContext', () => ({
  useBreadcrumb: () => ({
    setOverride: mockSetOverride,
    clearOverride: mockClearOverride,
    overrides: {},
  }),
}))

describe('SessionFileManager page', () => {
  beforeEach(() => {
    mockUseActiveConnections.mockReset()
    mockUseCurrentUser.mockReset()
    mockSetOverride.mockReset()
    mockClearOverride.mockReset()
    workspaceMock.mockClear()
    mockUseCurrentUser.mockReturnValue({
      data: {
        id: 'usr-1',
        username: 'alice',
        email: 'alice@example.com',
        first_name: 'Alice',
        last_name: 'Doe',
        is_root: false,
        is_active: true,
      },
    })
  })

  it('renders session details when data is available', () => {
    mockUseActiveConnections.mockReturnValue({
      data: [
        {
          id: 'sess-1',
          connection_id: 'conn-1',
          connection_name: 'Primary Server',
          user_id: 'usr-1',
          user_name: 'Alice',
          protocol_id: 'ssh',
          started_at: '2024-01-01T00:00:00Z',
          last_seen_at: '2024-01-01T01:00:00Z',
          metadata: {},
          participants: {
            'usr-1': {
              session_id: 'sess-1',
              user_id: 'usr-1',
              user_name: 'Alice',
              role: 'owner',
              access_mode: 'write',
              joined_at: '2024-01-01T00:00:00Z',
            },
          },
          write_holder: 'usr-1',
        },
      ],
      isLoading: false,
      isError: false,
    })

    render(
      <MemoryRouter initialEntries={['/active-sessions/sess-1']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SessionFileManager />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('Primary Server')).toBeInTheDocument()
    expect(screen.getByTestId('sftp-workspace-mock')).toBeInTheDocument()
    expect(workspaceMock).toHaveBeenCalled()
    const props = workspaceMock.mock.calls.at(-1)?.[0] as Record<string, unknown>
    expect(props?.sessionId).toBe('sess-1')
    expect(props?.canWrite).toBe(true)
    expect(props?.currentUserId).toBe('usr-1')
  })

  it('renders empty state when session is missing', () => {
    mockUseActiveConnections.mockReturnValue({ data: [], isLoading: false, isError: false })
    mockUseCurrentUser.mockReturnValue({ data: null })

    render(
      <MemoryRouter initialEntries={['/active-sessions/unknown']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SessionFileManager />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('Session unavailable')).toBeInTheDocument()
  })

  it('shows disabled state when SFTP is not supported', () => {
    mockUseActiveConnections.mockReturnValue({
      data: [
        {
          id: 'sess-2',
          connection_id: 'conn-2',
          connection_name: 'Restricted Server',
          user_id: 'usr-1',
          user_name: 'Alice',
          protocol_id: 'ssh',
          started_at: '2024-01-02T00:00:00Z',
          last_seen_at: '2024-01-02T01:00:00Z',
          metadata: {
            sftp_enabled: false,
          },
          participants: {},
        },
      ],
      isLoading: false,
      isError: false,
    })

    render(
      <MemoryRouter initialEntries={['/active-sessions/sess-2']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SessionFileManager />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('SFTP disabled')).toBeInTheDocument()
    expect(workspaceMock).not.toHaveBeenCalled()
  })
})
