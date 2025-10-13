import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import * as React from 'react'
import { vi } from 'vitest'
import { ConnectionFormModal } from '@/components/connections/ConnectionFormModal'
import type { Protocol } from '@/types/protocols'
import type { ConnectionRecord } from '@/types/connections'

const mutateAsync = vi.fn()

vi.mock('@/hooks/useConnectionMutations', () => ({
  useConnectionMutations: () => ({
    create: {
      mutateAsync,
      isPending: false,
    },
  }),
}))

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => ({ hasPermission: () => true }),
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

vi.mock('@/components/vault/IdentitySelector', () => ({
  IdentitySelector: ({ onChange }: { onChange: (value: string | null) => void }) => (
    <button type="button" onClick={() => onChange('identity-1')}>
      Select identity
    </button>
  ),
}))

vi.mock('@/components/vault/IdentityFormModal', () => ({
  IdentityFormModal: () => null,
}))

const protocol: Protocol = {
  id: 'ssh',
  name: 'SSH',
  description: 'Secure shell',
  category: 'terminal',
  icon: 'terminal',
  sort_order: 1,
  features: [],
  metadata: {},
}

const connectionRecord: ConnectionRecord = {
  id: 'conn-1',
  name: 'SSH Example',
  protocol_id: 'ssh',
  description: 'demo',
  owner_user_id: null,
  folder_id: null,
  metadata: {},
  settings: {},
  identity_id: 'identity-1',
  team_id: null,
  last_used_at: null,
  targets: [],
  shares: [],
  share_summary: undefined,
  folder: undefined,
}

describe('ConnectionFormModal identity integration', () => {
  beforeEach(() => {
    mutateAsync.mockReset()
    mutateAsync.mockResolvedValue(connectionRecord)
  })

  it('requires an identity before submission', async () => {
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <ConnectionFormModal
          open
          onClose={vi.fn()}
          protocol={protocol}
          folders={[]}
          onSuccess={vi.fn()}
        />
      </QueryClientProvider>
    )

    fireEvent.change(screen.getByLabelText(/Connection name/i), {
      target: { value: 'Prod SSH' },
    })

    fireEvent.click(screen.getByRole('button', { name: /Create Connection/i }))

    expect(await screen.findByText(/Select or create a vault identity/i)).toBeInTheDocument()
    expect(mutateAsync).not.toHaveBeenCalled()
  })

  it('submits with selected identity', async () => {
    const handleSuccess = vi.fn()
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <ConnectionFormModal
          open
          onClose={vi.fn()}
          protocol={protocol}
          folders={[]}
          onSuccess={handleSuccess}
        />
      </QueryClientProvider>
    )

    fireEvent.change(screen.getByLabelText(/Connection name/i), {
      target: { value: 'Prod SSH' },
    })

    fireEvent.click(screen.getByText(/Select identity/i))

    fireEvent.click(screen.getByRole('button', { name: /Create Connection/i }))

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          identity_id: 'identity-1',
        })
      )
    })
    expect(handleSuccess).toHaveBeenCalledWith(connectionRecord)
  })
})
