import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { IdentityShareModal } from '@/components/vault/IdentityShareModal'

const grantMock = vi.fn()
const mockUseIdentitySharing = vi.fn(() => ({
  grant: { mutateAsync: grantMock, isPending: false },
}))

vi.mock('@/hooks/useIdentities', async (original) => {
  const actual = await original()
  return {
    ...actual,
    useIdentitySharing: () => mockUseIdentitySharing(),
  }
})

const mockUseUsers = vi.fn()
vi.mock('@/hooks/useUsers', () => ({
  useUsers: (params: unknown, options: unknown) => mockUseUsers(params, options),
}))

const mockUseTeams = vi.fn()
vi.mock('@/hooks/useTeams', () => ({
  useTeams: (options: unknown) => mockUseTeams(options),
}))

describe('IdentityShareModal', () => {
  beforeEach(() => {
    grantMock.mockReset()
    mockUseIdentitySharing.mockReturnValue({ grant: { mutateAsync: grantMock, isPending: false } })
    mockUseUsers.mockReturnValue({
      data: { data: [{ id: 'usr-1', username: 'alice', email: 'a@example.com' }] },
    })
    mockUseTeams.mockReturnValue({ data: { data: [{ id: 'team-1', name: 'Ops' }] } })
  })

  it('submits share request for selected user', async () => {
    render(<IdentityShareModal identityId="id-1" open onClose={vi.fn()} />)

    const select = screen.getByLabelText(/User/i)
    fireEvent.change(select, { target: { value: 'usr-1' } })

    fireEvent.click(screen.getByRole('button', { name: /Share identity/i }))

    await waitFor(() => {
      expect(grantMock).toHaveBeenCalledWith({
        principal_type: 'user',
        principal_id: 'usr-1',
        permission: 'use',
        expires_at: undefined,
      })
    })
  })
})
