import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { UserForm } from '@/components/users/UserForm'
import type { UserRecord } from '@/types/users'

const mockCreate = vi.fn()
const mockUpdate = vi.fn()

vi.mock('@/hooks/useUsers', () => ({
  useUserMutations: () => ({
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

describe('UserForm', () => {
  beforeEach(() => {
    mockCreate.mockReset()
    mockUpdate.mockReset()
  })

  it('submits create payload', async () => {
    const handleSuccess = vi.fn()
    mockCreate.mockResolvedValueOnce({ id: 'usr-1' })
    render(<UserForm mode="create" onSuccess={handleSuccess} />)

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'carol' } })
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'carol@example.com' } })
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'SecurePass123!' } })

    fireEvent.click(screen.getByRole('button', { name: /create user/i }))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          username: 'carol',
          email: 'carol@example.com',
          password: 'SecurePass123!',
        })
      )
    })
    expect(handleSuccess).toHaveBeenCalled()
  })

  it('submits update payload when editing', async () => {
    const user: UserRecord = {
      id: 'usr-2',
      username: 'dave',
      email: 'dave@example.com',
      is_active: true,
      is_root: false,
    }
    mockUpdate.mockResolvedValueOnce(user)
    render(<UserForm mode="edit" user={user} />)

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'davey' } })
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith({
        userId: 'usr-2',
        payload: expect.objectContaining({ username: 'davey' }),
      })
    })
  })
})
