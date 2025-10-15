import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProtocolSettings } from '../ProtocolSettings'
import type { SSHProtocolSettings } from '@/types/protocol-settings'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { toast } from 'sonner'

const hookMock = vi.fn()
vi.mock('@/hooks/useProtocolSettings', () => ({
  useSSHProtocolSettings: () => hookMock(),
}))

vi.mock('@/components/ui/Select', () => {
  const React = require('react')
  const SelectContext = React.createContext({
    value: '',
    onChange: (_value: string) => {},
    id: undefined as string | undefined,
    setId: (_id?: string) => {},
  })

  const Select = ({ value, onValueChange, children }: any) => {
    const [triggerId, setTriggerId] = React.useState<string | undefined>()
    return (
      <SelectContext.Provider
        value={{ value, onChange: onValueChange, id: triggerId, setId: setTriggerId }}
      >
        {children}
      </SelectContext.Provider>
    )
  }

  const SelectTrigger = ({ children, id }: any) => {
    const ctx = React.useContext(SelectContext)
    React.useEffect(() => {
      ctx.setId(id)
    }, [ctx, id])
    return <label htmlFor={id}>{children}</label>
  }

  const SelectContent = ({ children }: any) => {
    const ctx = React.useContext(SelectContext)
    return (
      <select
        id={ctx.id}
        value={ctx.value}
        onChange={(event) => ctx.onChange?.(event.target.value)}
      >
        {children}
      </select>
    )
  }

  const SelectItem = ({ value, children }: any) => <option value={value}>{children}</option>

  const SelectValue = ({ placeholder }: any) => <span>{placeholder}</span>

  return { Select, SelectTrigger, SelectContent, SelectItem, SelectValue }
})

beforeAll(() => {
  if (!HTMLElement.prototype.hasPointerCapture) {
    Object.defineProperty(HTMLElement.prototype, 'hasPointerCapture', {
      configurable: true,
      value: () => false,
    })
  }
  if (!HTMLElement.prototype.releasePointerCapture) {
    Object.defineProperty(HTMLElement.prototype, 'releasePointerCapture', {
      configurable: true,
      value: () => undefined,
    })
  }
  if (!HTMLElement.prototype.scrollIntoView) {
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: () => undefined,
    })
  }
})

describe('ProtocolSettings', () => {
  beforeEach(() => {
    hookMock.mockReset()
    vi.mocked(toast.success).mockReset()
    vi.mocked(toast.error).mockReset()
  })

  const defaultSettings: SSHProtocolSettings = {
    recording: {
      mode: 'optional',
      storage: 'filesystem',
      retention_days: 90,
      require_consent: true,
    },
  }

  it('submits updated recording defaults', async () => {
    const mutateAsync = vi.fn().mockResolvedValue(defaultSettings)
    hookMock.mockReturnValue({
      data: defaultSettings,
      isLoading: false,
      isFetching: false,
      update: {
        mutateAsync,
        isPending: false,
        isSuccess: false,
      },
    })

    const user = userEvent.setup()
    render(<ProtocolSettings />)

    expect(screen.getByDisplayValue('90')).toBeInTheDocument()

    const modeSelect = screen.getByLabelText('Recording mode') as HTMLSelectElement
    await user.selectOptions(modeSelect, 'forced')

    const storageSelect = screen.getByLabelText('Recording storage') as HTMLSelectElement
    await user.selectOptions(storageSelect, 's3')

    const retentionInput = screen.getByLabelText('Retention (days)') as HTMLInputElement
    await user.clear(retentionInput)
    await user.type(retentionInput, '45')

    await user.click(screen.getByLabelText('Require participant consent'))

    await user.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        recording: {
          mode: 'forced',
          storage: 's3',
          retention_days: 45,
          require_consent: false,
        },
      })
    )
    expect(toast.success).toHaveBeenCalledWith('Protocol settings updated', {
      description: 'Recording defaults saved successfully.',
    })
  })

  it('surfaces errors when update fails', async () => {
    const mutateAsync = vi.fn().mockRejectedValue(new Error('network failure'))
    hookMock.mockReturnValue({
      data: defaultSettings,
      isLoading: false,
      isFetching: false,
      update: {
        mutateAsync,
        isPending: false,
        isSuccess: false,
      },
    })

    const user = userEvent.setup()
    render(<ProtocolSettings />)

    await user.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled())
    expect(toast.error).toHaveBeenCalledWith('Failed to update recording defaults', {
      description: 'network failure',
    })
  })
})
