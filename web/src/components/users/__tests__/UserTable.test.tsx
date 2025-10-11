import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { UserTable } from '@/components/users/UserTable'
import type { UserRecord } from '@/types/users'

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

describe('UserTable', () => {
  const users: UserRecord[] = [
    {
      id: 'usr-1',
      username: 'alice',
      email: 'alice@example.com',
      is_root: false,
      is_active: true,
      roles: [{ id: 'role-admin', name: 'Admin' }],
      teams: [],
    },
    {
      id: 'usr-2',
      username: 'bob',
      email: 'bob@example.com',
      is_root: false,
      is_active: false,
      roles: [],
      teams: [],
    },
  ]

  it('renders user rows and handles selection', () => {
    const handleSelection = vi.fn()
    render(
      <UserTable
        users={users}
        page={1}
        perPage={20}
        onPageChange={() => {}}
        onSelectionChange={handleSelection}
      />
    )

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()

    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[1])

    expect(handleSelection).toHaveBeenLastCalledWith(['usr-1'])
  })
})
