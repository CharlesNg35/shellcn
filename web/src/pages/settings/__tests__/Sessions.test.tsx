import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, beforeEach, vi } from 'vitest'

import { Sessions } from '../Sessions'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SessionRecordingSummary } from '@/types/session-recording'
import { usePermissions } from '@/hooks/usePermissions'
import { useTeams } from '@/hooks/useTeams'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useSessionRecordings, useDeleteSessionRecording } from '@/hooks/useSessionRecordings'
import { downloadSessionRecording } from '@/lib/api/session-recordings'

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: vi.fn(),
}))

vi.mock('@/hooks/useTeams', () => ({
  useTeams: vi.fn(),
}))

vi.mock('@/hooks/useActiveConnections', () => ({
  useActiveConnections: vi.fn(),
}))

vi.mock('@/hooks/useSessionRecordings', () => ({
  useSessionRecordings: vi.fn(),
  useDeleteSessionRecording: vi.fn(),
  SESSION_RECORDINGS_QUERY_KEY: ['session-recordings'],
}))

vi.mock('@/lib/api/session-recordings', () => ({
  downloadSessionRecording: vi.fn(),
}))

vi.mock('@/lib/utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockUsePermissions = vi.mocked(usePermissions)
const mockUseTeams = vi.mocked(useTeams)
const mockUseActiveConnections = vi.mocked(useActiveConnections)
const mockUseSessionRecordings = vi.mocked(useSessionRecordings)
const mockUseDelete = vi.mocked(useDeleteSessionRecording)
const mockDownloadRecording = vi.mocked(downloadSessionRecording)

describe('Sessions settings page', () => {
  const activeSessions: ActiveConnectionSession[] = [
    {
      id: 'sess-1',
      connection_id: 'conn-1',
      connection_name: 'Build Server',
      user_id: 'user-1',
      user_name: 'Alice',
      team_id: 'team-a',
      protocol_id: 'ssh',
      started_at: new Date('2024-05-01T10:00:00Z').toISOString(),
      last_seen_at: new Date('2024-05-01T10:05:00Z').toISOString(),
      participants: {},
      metadata: {},
    },
  ] as unknown as ActiveConnectionSession[]

  const recordings: SessionRecordingSummary[] = [
    {
      record_id: 'rec-1',
      session_id: 'sess-1',
      connection_id: 'conn-1',
      connection_name: 'Build Server',
      protocol_id: 'ssh',
      owner_user_id: 'user-1',
      owner_user_name: 'Alice',
      team_id: 'team-a',
      created_by_user_id: 'user-2',
      created_by_user_name: 'Bob',
      storage_kind: 'filesystem',
      storage_path: 'rec-1.cast.gz',
      size_bytes: 1024,
      duration_seconds: 90,
      created_at: new Date('2024-05-01T09:55:00Z').toISOString(),
      retention_until: new Date('2024-06-01T00:00:00Z').toISOString(),
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    mockUsePermissions.mockReturnValue({
      hasPermission: (permission: string) =>
        [
          'session.active.view_all',
          'session.active.view_team',
          'session.recording.view',
          'session.recording.view_team',
          'session.recording.view_all',
          'session.recording.delete',
          'team.view',
        ].includes(permission),
    })
    mockUseTeams.mockReturnValue({
      data: { data: [{ id: 'team-a', name: 'Team A' }] },
      isLoading: false,
    })
    mockUseActiveConnections.mockReturnValue({
      data: activeSessions,
      isLoading: false,
      refetch: vi.fn(),
    })
    mockUseSessionRecordings.mockReturnValue({
      data: { data: recordings, meta: { total: recordings.length, page: 1, per_page: 20 } },
      isLoading: false,
      refetch: vi.fn(),
    })
    mockUseDelete.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    })
    mockDownloadRecording.mockResolvedValue(new Blob(['dummy'], { type: 'application/gzip' }))
  })

  it('renders active sessions and recordings tables', async () => {
    const user = userEvent.setup()
    const queryClient = new QueryClient()
    const { getByRole, getByText } = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Sessions />
        </MemoryRouter>
      </QueryClientProvider>
    )

    expect(getByText('Active Sessions', { selector: 'h3' })).toBeInTheDocument()
    expect(getByText('Build Server')).toBeInTheDocument()

    const recordingsTab = getByRole('tab', { name: 'Session Recordings' })
    await user.click(recordingsTab)
    expect(recordingsTab).toHaveAttribute('aria-selected', 'true')
    expect(getByText('Bob')).toBeInTheDocument()
  })

  it('requests active sessions with team or all scope selections', () => {
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Sessions />
        </MemoryRouter>
      </QueryClientProvider>
    )

    const initialCall = mockUseActiveConnections.mock.calls[0]?.[0]
    expect(initialCall?.scope).toBe('team')

    const scopeTrigger = screen.getAllByRole('combobox')[0]
    fireEvent.click(scopeTrigger)
    fireEvent.click(screen.getByRole('option', { name: 'All sessions' }))

    const updatedCall = mockUseActiveConnections.mock.calls.at(-1)?.[0]
    expect(updatedCall?.scope).toBe('all')
  })

  it('allows downloading and deleting recordings', async () => {
    const deleteMutation = vi.fn().mockResolvedValue(undefined)
    mockUseDelete.mockReturnValue({ mutateAsync: deleteMutation, isPending: false })

    const user = userEvent.setup()
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Sessions />
        </MemoryRouter>
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('tab', { name: 'Session Recordings' }))

    const downloadButton = screen.getByTitle('Download recording')
    await user.click(downloadButton)

    await waitFor(() => {
      expect(mockDownloadRecording).toHaveBeenCalledWith('rec-1')
    })

    const deleteButton = screen.getByTitle('Delete recording')
    await user.click(deleteButton)

    await waitFor(() => {
      expect(deleteMutation).toHaveBeenCalledWith('rec-1')
    })
  })
})
