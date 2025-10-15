import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SshWorkspace from '@/pages/sessions/SshWorkspace'

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

vi.mock('@/contexts/BreadcrumbContext', () => ({
  useBreadcrumb: () => ({
    setOverride: mockSetOverride,
    clearOverride: mockClearOverride,
    overrides: {},
  }),
}))

const terminalMock = vi.fn(() => <div data-testid="ssh-terminal-mock" />)
const sftpMock = vi.fn(() => <div data-testid="sftp-workspace-mock" />)

vi.mock('@/components/workspace/SshTerminal', () => ({
  SshTerminal: (...args: unknown[]) => terminalMock(...args),
  default: (...args: unknown[]) => terminalMock(...args),
}))

vi.mock('@/components/workspace/SftpWorkspace', () => ({
  SftpWorkspace: (...args: unknown[]) => sftpMock(...args),
  default: (...args: unknown[]) => sftpMock(...args),
}))

describe('SshWorkspace page', () => {
  beforeEach(() => {
    mockUseActiveConnections.mockReset()
    mockUseCurrentUser.mockReset()
    mockSetOverride.mockReset()
    mockClearOverride.mockReset()
    terminalMock.mockReset()
    sftpMock.mockReset()

    mockUseCurrentUser.mockReturnValue({
      data: {
        id: 'usr-1',
        first_name: 'Alice',
        last_name: 'Smith',
        username: 'alice',
        email: 'alice@example.com',
      },
    })
  })

  it('renders workspace with terminal and sftp tabs', () => {
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
          participants: {},
        },
      ],
      isLoading: false,
      isError: false,
    })

    render(
      <MemoryRouter initialEntries={['/active-sessions/sess-1']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SshWorkspace />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('Primary Server')).toBeInTheDocument()
    expect(screen.getByTestId('ssh-terminal-mock')).toBeInTheDocument()
    expect(screen.getByTestId('workspace-tab-terminal')).toBeInTheDocument()
    expect(screen.getByTestId('workspace-tab-sftp')).toBeInTheDocument()
    expect(terminalMock).toHaveBeenCalled()
    expect((terminalMock.mock.calls.at(-1)?.[0] as { sessionId: string })?.sessionId).toBe('sess-1')

    fireEvent.click(screen.getByTestId('workspace-tab-sftp'))
    expect(sftpMock).toHaveBeenCalled()
  })

  it('allows selecting layout columns and persists selection', () => {
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
          participants: {},
        },
      ],
      isLoading: false,
      isError: false,
    })

    render(
      <MemoryRouter initialEntries={['/active-sessions/sess-1']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SshWorkspace />} />
        </Routes>
      </MemoryRouter>
    )

    const grid = screen.getByTestId('terminal-grid')
    expect(grid).toHaveAttribute('data-columns', '1')

    const [threeColumnButton] = screen.getAllByRole('button', { name: '3 columns' })
    fireEvent.click(threeColumnButton)
    expect(grid).toHaveAttribute('data-columns', '3')
  })

  it('renders fallback when session missing', () => {
    mockUseActiveConnections.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    })
    mockUseCurrentUser.mockReturnValue({ data: null })

    render(
      <MemoryRouter initialEntries={['/active-sessions/unknown']}>
        <Routes>
          <Route path="/active-sessions/:sessionId" element={<SshWorkspace />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('Session unavailable')).toBeInTheDocument()
  })
})
