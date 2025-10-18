import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import { SessionShareDialog } from '@/pages/sessions/ssh-workspace/SessionShareDialog'
import type { SessionParticipantsSummary } from '@/types/connections'

const mockUseSessionParticipants = vi.fn()
const mockUseSessionParticipantMutations = vi.fn()
const mockUseUsers = vi.fn()

vi.mock('@/hooks/useSessionParticipants', () => ({
  useSessionParticipants: (...args: unknown[]) => mockUseSessionParticipants(...args),
  useSessionParticipantMutations: (...args: unknown[]) =>
    mockUseSessionParticipantMutations(...args),
}))

vi.mock('@/hooks/useUsers', () => ({
  useUsers: (...args: unknown[]) => mockUseUsers(...args),
}))

describe('SessionShareDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    const participants: SessionParticipantsSummary = {
      session_id: 'sess-1',
      connection_id: 'conn-1',
      owner_user_id: 'owner-1',
      owner_user_name: 'Owner',
      write_holder: 'owner-1',
      participants: [
        {
          session_id: 'sess-1',
          user_id: 'owner-1',
          user_name: 'Owner',
          role: 'owner',
          access_mode: 'write',
          joined_at: '2024-01-01T00:00:00Z',
          is_owner: true,
          is_write_holder: false,
        },
        {
          session_id: 'sess-1',
          user_id: 'writer-1',
          user_name: 'Writer',
          role: 'participant',
          access_mode: 'read',
          joined_at: '2024-01-01T00:05:00Z',
          is_owner: false,
          is_write_holder: false,
        },
      ],
    }

    mockUseSessionParticipants.mockReturnValue({
      data: participants,
      isLoading: false,
    })

    mockUseSessionParticipantMutations.mockReturnValue({
      invite: {
        mutateAsync: vi.fn().mockResolvedValue(undefined),
        isPending: false,
      },
      remove: {
        mutate: vi.fn(),
        isPending: false,
      },
      grantWrite: {
        mutate: vi.fn(),
        isPending: false,
      },
      relinquishWrite: {
        mutate: vi.fn(),
        isPending: false,
      },
    })

    mockUseUsers.mockReturnValue({
      data: {
        data: [
          {
            id: 'user-2',
            username: 'charlie',
            email: 'charlie@example.com',
            first_name: 'Charlie',
            last_name: 'Day',
          },
        ],
      },
      isLoading: false,
    })
  })

  it('renders participants and triggers grant write action', async () => {
    render(
      <SessionShareDialog
        sessionId="sess-1"
        open
        onClose={() => undefined}
        session={{
          id: 'sess-1',
          connection_id: 'conn-1',
          user_id: 'owner-1',
          protocol_id: 'ssh',
          started_at: '2024-01-01T00:00:00Z',
          last_seen_at: '2024-01-01T00:05:00Z',
          participants: {
            'owner-1': {
              session_id: 'sess-1',
              user_id: 'owner-1',
              user_name: 'Owner',
              role: 'owner',
              access_mode: 'write',
              joined_at: '2024-01-01T00:00:00Z',
            },
            'writer-1': {
              session_id: 'sess-1',
              user_id: 'writer-1',
              user_name: 'Writer',
              role: 'participant',
              access_mode: 'read',
              joined_at: '2024-01-01T00:05:00Z',
            },
          },
          connection_name: 'Primary',
          owner_user_id: 'owner-1',
          owner_user_name: 'Owner',
        }}
        currentUserId="owner-1"
        canShare
        canGrantWrite
      />
    )

    expect(screen.getAllByText('Owner').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Writer').length).toBeGreaterThan(0)

    const grantButton = screen.getByRole('button', { name: /grant write/i })
    fireEvent.click(grantButton)

    const mutations = mockUseSessionParticipantMutations.mock.results[0]!.value
    expect(mutations.grantWrite.mutate).toHaveBeenCalledWith({ userId: 'writer-1' })
  })

  it('invites a participant when form submitted', async () => {
    render(
      <SessionShareDialog
        sessionId="sess-1"
        open
        onClose={() => undefined}
        session={{
          id: 'sess-1',
          connection_id: 'conn-1',
          user_id: 'owner-1',
          protocol_id: 'ssh',
          started_at: '2024-01-01T00:00:00Z',
          last_seen_at: '2024-01-01T00:05:00Z',
          participants: {},
          connection_name: 'Primary',
          owner_user_id: 'owner-1',
        }}
        currentUserId="owner-1"
        canShare
        canGrantWrite
      />
    )

    const inviteButton = screen.getByRole('button', { name: /invite participant/i })
    fireEvent.click(inviteButton)

    const mutations = mockUseSessionParticipantMutations.mock.results[0]!.value
    expect(mutations.invite.mutateAsync).toHaveBeenCalledWith({ user_id: 'user-2' })
  })
})
