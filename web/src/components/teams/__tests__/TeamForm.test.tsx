import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { TeamForm } from '@/components/teams/TeamForm'
import type { TeamRecord } from '@/types/teams'

const mockCreate = vi.fn()
const mockUpdate = vi.fn()

vi.mock('@/hooks/useTeams', () => ({
  useTeamMutations: () => ({
    create: {
      mutateAsync: mockCreate,
      isPending: false,
    },
    update: {
      mutateAsync: mockUpdate,
      isPending: false,
    },
  }),
}))

describe('TeamForm', () => {
  beforeEach(() => {
    mockCreate.mockReset()
    mockUpdate.mockReset()
  })

  it('shows validation message when name is missing on create', async () => {
    const user = userEvent.setup()

    render(<TeamForm mode="create" />)

    await user.click(screen.getByRole('button', { name: /create team/i }))

    expect(mockCreate).not.toHaveBeenCalled()
    expect(screen.getByText(/team name must be at least 2 characters/i)).toBeInTheDocument()
  })

  it('prefills existing values in edit mode', () => {
    const team: TeamRecord = {
      id: 'team-2',
      name: 'Platform',
      description: 'Core infrastructure',
    }

    render(<TeamForm mode="edit" team={team} />)

    expect(screen.getByDisplayValue('Platform')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Core infrastructure')).toBeInTheDocument()
    expect(mockUpdate).not.toHaveBeenCalled()
  })
})
