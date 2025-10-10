import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import type { UseMutationResult } from '@tanstack/react-query'
import { TeamMembersManager } from '@/components/teams/TeamMembersManager'
import type { TeamMember, TeamRecord } from '@/types/teams'

const { mockUseUsers } = vi.hoisted(() => ({
  mockUseUsers: vi.fn(),
}))

vi.mock('@/hooks/useUsers', () => ({
  useUsers: mockUseUsers,
}))

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => ({
    permissions: [],
    hasPermission: () => true,
    hasAnyPermission: () => true,
    hasAllPermissions: () => true,
    isLoading: false,
    refetch: vi.fn(),
  }),
}))

function createMutationMock() {
  return {
    mutateAsync: vi.fn().mockResolvedValue(true),
    isPending: false,
  } as unknown as UseMutationResult<boolean, unknown, { teamId: string; userId: string }>
}

describe('TeamMembersManager', () => {
  const team: TeamRecord = {
    id: 'team-1',
    name: 'Security',
    description: 'Security team',
  }

  const members: TeamMember[] = [
    {
      id: 'usr-member',
      username: 'bob',
      email: 'bob@example.com',
      is_active: true,
    },
  ]

  beforeEach(() => {
    mockUseUsers.mockReset()
  })

  it('renders members and triggers removal', async () => {
    mockUseUsers.mockReturnValue({
      data: {
        data: [],
      },
      isLoading: false,
    })

    const addMemberMutation = createMutationMock()
    const removeMemberMutation = createMutationMock()

    const { container } = render(
      <TeamMembersManager
        team={team}
        members={members}
        addMemberMutation={addMemberMutation}
        removeMemberMutation={removeMemberMutation}
      />
    )

    expect(container.textContent).toContain('bob@example.com')

    const removeButton = await screen.findByRole('button', { name: /remove bob/i })
    fireEvent.click(removeButton)

    await waitFor(() => {
      expect(removeMemberMutation.mutateAsync).toHaveBeenCalledWith({
        teamId: 'team-1',
        userId: 'usr-member',
      })
    })
  })
})
