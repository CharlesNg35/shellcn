import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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
    update: {
      mutateAsync,
      isPending: false,
    },
  }),
}))

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => ({ hasPermission: () => true }),
}))

vi.mock('@/hooks/useConnectionTemplate', () => ({
  useConnectionTemplate: () => ({
    data: {
      driverId: 'ssh',
      version: '2025-01-15',
      displayName: 'SSH Connection',
      description: 'Configure host and port',
      metadata: { requires_identity: true },
      sections: [
        {
          id: 'endpoint',
          label: 'Endpoint',
          description: 'Where to connect',
          fields: [
            {
              key: 'host',
              label: 'Host',
              type: 'string',
              required: true,
              placeholder: 'server.example.com',
              validation: {},
              dependencies: [],
            },
            {
              key: 'port',
              label: 'Port',
              type: 'target_port',
              required: false,
              default: 22,
              validation: { min: 1, max: 65535 },
              dependencies: [],
            },
          ],
        },
      ],
    },
    isLoading: false,
  }),
}))

vi.mock('@/components/ui/Select', () => {
  const SelectContent = Object.assign(
    ({ children }: { children: React.ReactNode }) => <>{children}</>,
    { displayName: 'MockSelectContent' }
  )

  const SelectItem = Object.assign(
    ({ value, children }: { value: string; children: React.ReactNode }) => (
      <option value={value}>{children}</option>
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
  }: {
    value?: string | null
    onValueChange?: (value: string) => void
    children: React.ReactNode
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
        value={value ?? ''}
        onChange={(event) => onValueChange?.(event.target.value)}
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
  module: 'ssh',
  description: 'Secure shell',
  category: 'terminal',
  icon: 'terminal',
  defaultPort: 22,
  sortOrder: 1,
  features: [],
  capabilities: {
    terminal: true,
    desktop: false,
    file_transfer: true,
    clipboard: false,
    session_recording: false,
    metrics: false,
    reconnect: true,
    extras: {},
  },
  driverEnabled: true,
  configEnabled: true,
  available: true,
  permissions: [],
  identityRequired: true,
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

describe('ConnectionFormModal dynamic template', () => {
  beforeEach(() => {
    mutateAsync.mockReset()
    mutateAsync.mockResolvedValue(connectionRecord)
  })

  it('requires identity when template indicates it is required', async () => {
    const user = userEvent.setup()
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

    await screen.findByLabelText(/Connection name/i)

    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Prod SSH')

    await user.type(screen.getByLabelText(/Host/i), 'prod.internal')

    await user.click(screen.getByRole('button', { name: /Create Connection/i }))

    await screen.findByText(/Select or create a vault identity/i)
  })

  it('submits template fields when identity is selected', async () => {
    const user = userEvent.setup()
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

    await screen.findByLabelText(/Connection name/i)

    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Prod SSH')

    await user.type(screen.getByLabelText(/Host/i), 'prod.internal')

    await user.click(screen.getByRole('button', { name: /Select identity/i }))

    await user.click(screen.getByRole('button', { name: /Create Connection/i }))

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalled()
    })

    const payload = mutateAsync.mock.calls[0][0]
    expect(payload.fields).toEqual({ host: 'prod.internal', port: 22 })
    expect(payload.identity_id).toBe('identity-1')
  })
})
