import { fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SshWorkspace from '@/pages/sessions/SshWorkspace'
import { PERMISSIONS } from '@/constants/permissions'

const mockSessionParticipants = vi.fn()
const mockSessionParticipantMutations = vi.fn()

vi.mock('@/hooks/useSessionParticipants', () => ({
  useSessionParticipants: (...args: unknown[]) => mockSessionParticipants(...args),
  useSessionParticipantMutations: (...args: unknown[]) => mockSessionParticipantMutations(...args),
}))

const mockUseUsers = vi.fn()

vi.mock('@/hooks/useUsers', () => ({
  useUsers: (...args: unknown[]) => mockUseUsers(...args),
}))

const mockTabsStore = vi.hoisted(() => {
  const baseTabs = [
    {
      id: 'sess-1:terminal',
      sessionId: 'sess-1',
      type: 'terminal' as const,
      title: 'Terminal',
      closable: false,
    },
    {
      id: 'sess-1:sftp',
      sessionId: 'sess-1',
      type: 'sftp' as const,
      title: 'Files',
      closable: true,
    },
  ]

  return {
    state: {
      sessions: {
        'sess-1': {
          sessionId: 'sess-1',
          connectionId: 'conn-1',
          tabs: baseTabs,
          activeTabId: baseTabs[0]!.id,
          layoutColumns: 1,
          isFullscreen: false,
          lastFocusedAt: Date.now(),
        },
      },
      orderedSessionIds: ['sess-1'],
    },
    openSession: vi.fn(),
    ensureTab: vi.fn(() => baseTabs[1]),
    closeTab: vi.fn(),
    reorderTabs: vi.fn(),
    setActiveTab: vi.fn(),
    setLayoutColumns: vi.fn(),
    setFullscreen: vi.fn(),
    reset() {
      this.openSession.mockReset()
      this.ensureTab.mockReset()
      this.closeTab.mockReset()
      this.reorderTabs.mockReset()
      this.setActiveTab.mockReset()
      this.setLayoutColumns.mockReset()
      this.setFullscreen.mockReset()
      this.state.sessions['sess-1'].activeTabId = baseTabs[0]!.id
      this.state.sessions['sess-1'].layoutColumns = 1
      this.state.sessions['sess-1'].isFullscreen = false
    },
  }
})

const mockCommandPalette = vi.hoisted(() => ({
  isOpen: false,
  open: vi.fn(),
  close: vi.fn(),
  toggle: vi.fn(),
  paletteTabs: [
    {
      id: 'sess-1:terminal',
      label: 'Terminal',
      isActive: true,
      onSelect: vi.fn(),
    },
  ],
  paletteSessions: [],
}))

type MockTabsStoreState = typeof mockTabsStore.state & {
  openSession: typeof mockTabsStore.openSession
  ensureTab: typeof mockTabsStore.ensureTab
  closeTab: typeof mockTabsStore.closeTab
  reorderTabs: typeof mockTabsStore.reorderTabs
  setActiveTab: typeof mockTabsStore.setActiveTab
  setLayoutColumns: typeof mockTabsStore.setLayoutColumns
  setFullscreen: typeof mockTabsStore.setFullscreen
}

vi.mock('@/store/ssh-session-tabs-store', () => {
  const composeState = (): MockTabsStoreState => ({
    ...mockTabsStore.state,
    openSession: mockTabsStore.openSession,
    ensureTab: mockTabsStore.ensureTab,
    closeTab: mockTabsStore.closeTab,
    reorderTabs: mockTabsStore.reorderTabs,
    setActiveTab: mockTabsStore.setActiveTab,
    setLayoutColumns: mockTabsStore.setLayoutColumns,
    setFullscreen: mockTabsStore.setFullscreen,
  })

  const useSshWorkspaceTabsStore = <T,>(selector: (state: MockTabsStoreState) => T): T =>
    selector(composeState())

  useSshWorkspaceTabsStore.getState = () => ({
    ...mockTabsStore.state,
    openSession: mockTabsStore.openSession,
    ensureTab: mockTabsStore.ensureTab,
    closeTab: mockTabsStore.closeTab,
    reorderTabs: mockTabsStore.reorderTabs,
    setActiveTab: mockTabsStore.setActiveTab,
    setLayoutColumns: mockTabsStore.setLayoutColumns,
    setFullscreen: mockTabsStore.setFullscreen,
  })

  return {
    useSshWorkspaceTabsStore,
    selectSessionWorkspace: (sessionId: string) => () => mockTabsStore.state.sessions[sessionId],
    resetSshWorkspaceTabsStore: () => mockTabsStore.reset(),
  }
})

type MockWorkspaceStoreState = {
  sessions: Record<
    string,
    {
      transfers: Record<string, unknown>
      transferOrder: string[]
    }
  >
}

vi.mock('@/store/ssh-workspace-store', () => {
  const state: MockWorkspaceStoreState = {
    sessions: {
      'sess-1': {
        transfers: {},
        transferOrder: [],
      },
    },
  }
  const useSshWorkspaceStore = <T,>(selector: (mockState: MockWorkspaceStoreState) => T): T =>
    selector(state)
  return {
    useSshWorkspaceStore,
    resetSshWorkspaceStore: vi.fn(),
  }
})

vi.mock('@/pages/sessions/ssh-workspace/useActiveSshSession', () => ({
  useActiveSshSession: () => ({
    session: {
      id: 'sess-1',
      connection_id: 'conn-1',
      connection_name: 'Primary Server',
      user_id: 'usr-1',
      user_name: 'Alice',
      protocol_id: 'ssh',
      started_at: '2024-01-01T00:00:00Z',
      last_seen_at: '2024-01-01T01:00:00Z',
      metadata: {},
      participants: {},
    },
    activeSessions: [],
    isLoading: false,
    isError: false,
  }),
}))

vi.mock('@/pages/sessions/ssh-workspace/useSessionTabsLifecycle', () => ({
  useSessionTabsLifecycle: vi.fn(),
}))

vi.mock('@/pages/sessions/ssh-workspace/useWorkspaceSnippets', () => ({
  useWorkspaceSnippets: () => ({
    groups: [
      {
        label: 'Global snippets',
        snippets: [{ id: 'snp-1', name: 'List processes', description: 'Show running processes' }],
      },
    ],
    isLoading: false,
    snippetsAvailable: true,
    executeSnippet: vi.fn(),
    isExecuting: false,
  }),
}))

vi.mock('@/pages/sessions/ssh-workspace/useCommandPaletteState', () => ({
  useCommandPaletteState: () => mockCommandPalette,
}))

vi.mock('@/pages/sessions/ssh-workspace/useTerminalSearch', () => ({
  useTerminalSearch: () => ({
    overlay: { visible: false, query: '', direction: 'next' as const },
    isOpen: false,
    query: '',
    direction: 'next' as const,
    matched: true,
    toggle: vi.fn(),
    onQueryChange: vi.fn(),
    onDirectionChange: vi.fn(),
    onSubmit: vi.fn(),
    onResolved: vi.fn(),
    inputRef: { current: null },
  }),
}))

vi.mock('@/pages/sessions/ssh-workspace/useWorkspaceTelemetry', () => ({
  useWorkspaceTelemetry: () => ({
    fontSize: 14,
    setFontSize: vi.fn(),
    handleTerminalEvent: vi.fn(),
    latencyMs: null,
    lastActivityAt: null,
    zoomIn: vi.fn(),
    zoomOut: vi.fn(),
    zoomReset: vi.fn(),
  }),
}))

vi.mock('framer-motion', () => ({
  Reorder: {
    Group: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    Item: ({
      children,
      onClick,
      'data-testid': dataTestId,
    }: {
      children: ReactNode
      onClick?: () => void
      'data-testid'?: string
    }) => (
      <div data-testid={dataTestId} onClick={onClick} role="presentation">
        {children}
      </div>
    ),
  },
  AnimatePresence: ({ children }: { children: ReactNode }) => <>{children}</>,
  motion: { button: 'button' },
}))

const mockUseActiveConnections = vi.fn()
const mockUseCurrentUser = vi.fn()
const mockSetOverride = vi.fn()
const mockClearOverride = vi.fn()
const mockUseSnippets = vi.fn()
const mockExecuteSnippet = vi.fn()
const mockUsePermissions = vi.fn()

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

vi.mock('@/hooks/useSnippets', () => ({
  useSnippets: (...args: unknown[]) => mockUseSnippets(...args),
  useExecuteSnippet: () => ({ mutate: mockExecuteSnippet, isLoading: false, isPending: false }),
}))

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => mockUsePermissions(),
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
    mockTabsStore.reset()
    mockCommandPalette.open.mockReset()
    mockCommandPalette.close.mockReset()
    mockCommandPalette.toggle.mockReset()
    mockCommandPalette.paletteTabs[0]!.onSelect.mockReset()
    mockUseActiveConnections.mockReset()
    mockUseCurrentUser.mockReset()
    mockSetOverride.mockReset()
    mockClearOverride.mockReset()
    mockUseSnippets.mockReset()
    mockExecuteSnippet.mockReset()
    mockUsePermissions.mockReset()
    terminalMock.mockReset()
    sftpMock.mockReset()

    mockSessionParticipants.mockReturnValue({
      data: {
        session_id: 'sess-1',
        connection_id: 'conn-1',
        owner_user_id: 'usr-1',
        participants: [],
      },
      isLoading: false,
    })

    mockSessionParticipantMutations.mockReturnValue({
      invite: { mutateAsync: vi.fn(), isPending: false },
      remove: { mutate: vi.fn(), isPending: false },
      grantWrite: { mutate: vi.fn(), isPending: false },
      relinquishWrite: { mutate: vi.fn(), isPending: false },
    })

    mockUseUsers.mockReturnValue({
      data: { data: [] },
      isLoading: false,
    })

    mockUsePermissions.mockReturnValue({
      hasPermission: (permission: string) =>
        permission === PERMISSIONS.PROTOCOL.SSH.SFTP ||
        permission === PERMISSIONS.PROTOCOL.SSH.MANAGE_SNIPPETS,
    })

    mockUseCurrentUser.mockReturnValue({
      data: {
        id: 'usr-1',
        first_name: 'Alice',
        last_name: 'Smith',
        username: 'alice',
        email: 'alice@example.com',
      },
    })

    mockUseSnippets.mockReturnValue({
      data: [
        {
          id: 'snp-1',
          name: 'List processes',
          description: 'Show running processes',
          scope: 'global',
          command: 'ps aux',
        },
      ],
      isLoading: false,
      isError: false,
    })

    mockUseActiveConnections.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    })
  })

  function renderWorkspace(initialPath = '/active-sessions/sess-1') {
    const queryClient = new QueryClient()
    return render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialPath]}>
          <Routes>
            <Route path="/active-sessions/:sessionId" element={<SshWorkspace />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    )
  }

  it('renders terminal, toolbar, and status elements', async () => {
    renderWorkspace()

    expect(screen.getByText('Primary Server')).toBeInTheDocument()
    expect(await screen.findByTestId('ssh-terminal-mock')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /change layout/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /file manager/i })).toBeInTheDocument()
  })

  it('opens file manager tab when toolbar button is pressed', async () => {
    renderWorkspace()

    fireEvent.click(screen.getByRole('button', { name: /file manager/i }))
    expect(mockTabsStore.ensureTab).toHaveBeenCalledWith('sess-1', 'sftp', {
      title: 'Files',
      closable: true,
    })
    expect(mockTabsStore.setActiveTab).toHaveBeenCalledWith('sess-1', 'sess-1:sftp')
    expect(await screen.findByTestId('sftp-workspace-mock')).toBeInTheDocument()
  })

  it('opens command palette via toolbar button', () => {
    renderWorkspace()

    fireEvent.click(screen.getByLabelText(/open command palette/i))
    expect(mockCommandPalette.open).toHaveBeenCalledTimes(1)
  })
})
