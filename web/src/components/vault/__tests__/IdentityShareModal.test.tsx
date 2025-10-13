import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as React from 'react'
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

vi.mock('@/components/ui/Select', () => {
  const SelectContent = Object.assign(
    ({ children }: { children: React.ReactNode }) => <>{children}</>,
    { displayName: 'MockSelectContent' }
  )

  const SelectItem = Object.assign(
    ({
      value,
      children,
      disabled,
    }: {
      value: string
      children: React.ReactNode
      disabled?: boolean
    }) => (
      <option value={value} disabled={disabled}>
        {children}
      </option>
    ),
    { displayName: 'MockSelectItem' }
  )

  const SelectTrigger = Object.assign(
    ({ children }: { children: React.ReactNode }) => <>{children}</>,
    { displayName: 'MockSelectTrigger' }
  )

  const SelectValue = () => null

  const Select = ({
    value,
    onValueChange,
    children,
    ...rest
  }: {
    value: string
    onValueChange?: (next: string) => void
    children: React.ReactNode
    [key: string]: unknown
  }) => {
    const options: React.ReactNode[] = []
    React.Children.forEach(children, (child) => {
      if (!child) {
        return
      }
      if ((child as React.ReactElement).type?.displayName === 'MockSelectContent') {
        React.Children.forEach((child as React.ReactElement).props.children, (optionChild) => {
          if (optionChild) {
            options.push(optionChild)
          }
        })
      }
    })

    return (
      <select
        data-testid="mock-select"
        value={value}
        onChange={(event) => onValueChange?.(event.target.value)}
        {...rest}
      >
        {options}
      </select>
    )
  }

  return {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  }
})

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

    const select = screen.getAllByTestId('mock-select')[0]
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
