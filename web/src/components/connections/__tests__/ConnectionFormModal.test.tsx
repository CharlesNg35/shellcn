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

vi.mock('@/lib/api/protocol-settings', () => ({
  fetchSSHProtocolSettings: vi.fn().mockResolvedValue({
    session: {
      concurrent_limit: 2,
      idle_timeout_minutes: 30,
      enable_sftp: true,
    },
    terminal: {
      theme_mode: 'auto',
      font_family: 'Fira Code',
      font_size: 14,
      scrollback_limit: 1200,
      enable_webgl: true,
    },
    recording: {
      mode: 'optional',
      storage: 'filesystem',
      retention_days: 0,
      require_consent: false,
    },
    collaboration: {
      allow_sharing: true,
      restrict_write_to_admins: false,
    },
  }),
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

    await screen.findByPlaceholderText('Production SSH')

    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Prod SSH')

    await user.click(screen.getByRole('button', { name: /Create Connection/i }))

    expect(await screen.findByText(/Select or create a vault identity/i)).toBeInTheDocument()
    expect(mutateAsync).not.toHaveBeenCalled()
  })

  it('submits with selected identity', async () => {
    const handleSuccess = vi.fn()
    const queryClient = new QueryClient()
    const user = userEvent.setup()
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

    await screen.findByPlaceholderText('Production SSH')

    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Prod SSH')

    await user.click(screen.getByText(/Select identity/i))

    await user.click(screen.getByRole('button', { name: /Create Connection/i }))

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          identity_id: 'identity-1',
          settings: expect.objectContaining({ recording_enabled: false }),
        })
      )
    })
    expect(handleSuccess).toHaveBeenCalledWith(connectionRecord)
  })

  it('allows overriding session defaults before submission', async () => {
    const queryClient = new QueryClient()
    const user = userEvent.setup()
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

    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Prod SSH')

    await user.click(screen.getByText(/Select identity/i))

    await user.click(screen.getByLabelText(/Customise session values/i))

    const concurrentInput = screen.getByLabelText(/Concurrent sessions/i)
    await user.clear(concurrentInput)
    await user.type(concurrentInput, '5')

    const idleInput = screen.getByLabelText(/Idle timeout/i)
    await user.clear(idleInput)
    await user.type(idleInput, '10')

    await user.click(screen.getByLabelText(/Allow SFTP/i))

    await user.click(screen.getByRole('button', { name: /Create Connection/i }))

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          settings: expect.objectContaining({
            concurrent_limit: 5,
            idle_timeout_minutes: 10,
            enable_sftp: false,
          }),
        })
      )
    })
  })

  it('triggers update mutation in edit mode', async () => {
    const existing: ConnectionRecord = {
      ...connectionRecord,
      metadata: { icon: 'terminal', color: '#ff0000' },
      settings: {
        concurrent_limit: 3,
        idle_timeout_minutes: 45,
        enable_sftp: true,
        recording_enabled: true,
      },
    }

    const handleSuccess = vi.fn()
    const queryClient = new QueryClient()
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={queryClient}>
        <ConnectionFormModal
          open
          onClose={vi.fn()}
          protocol={protocol}
          folders={[]}
          mode="edit"
          connection={existing}
          onSuccess={handleSuccess}
        />
      </QueryClientProvider>
    )

    await screen.findByDisplayValue('SSH Example')
    await user.clear(screen.getByLabelText(/Connection name/i))
    await user.type(screen.getByLabelText(/Connection name/i), 'Updated SSH')

    await user.click(screen.getByText(/Select identity/i))

    const sessionOverrideToggle = screen.getByLabelText(
      'Customise session values'
    ) as HTMLInputElement
    expect(sessionOverrideToggle.checked).toBe(true)

    await user.click(screen.getByRole('button', { name: /Save changes/i }))

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled())
    const lastCall = mutateAsync.mock.calls.at(-1)?.[0] as
      | { id: string; payload: Record<string, unknown> }
      | undefined
    expect(lastCall).toBeDefined()
    expect(lastCall?.id).toBe(existing.id)
    expect(lastCall?.payload).toMatchObject({
      name: 'Updated SSH',
      settings: expect.objectContaining({
        concurrent_limit: 3,
        idle_timeout_minutes: 45,
        enable_sftp: true,
        recording_enabled: true,
      }),
    })
    expect(handleSuccess).toHaveBeenCalledWith(connectionRecord)
  })
})
