import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProtocolSettings } from '../ProtocolSettings'
import type { SSHProtocolSettings } from '@/types/protocol-settings'

const hookMock = vi.fn()
vi.mock('@/hooks/useProtocolSettings', () => ({
  useSSHProtocolSettings: () => hookMock(),
}))

vi.mock('@/components/ui/Select', async () => {
  const React = await import('react')
  const SelectContext = React.createContext({
    value: '',
    onChange: (value: string) => value,
    id: undefined as string | undefined,
    setId: (value?: string) => value,
  })

  const Select = ({
    value,
    onValueChange,
    children,
  }: {
    value: string
    onValueChange?: (value: string) => void
    children: React.ReactNode
  }) => {
    const [triggerId, setTriggerId] = React.useState<string | undefined>()
    return (
      <SelectContext.Provider
        value={{ value, onChange: onValueChange, id: triggerId, setId: setTriggerId }}
      >
        {children}
      </SelectContext.Provider>
    )
  }

  const SelectTrigger = ({ children, id }: { children: React.ReactNode; id?: string }) => {
    const ctx = React.useContext(SelectContext)
    React.useEffect(() => {
      ctx.setId(id)
    }, [ctx, id])
    return <label htmlFor={id}>{children}</label>
  }

  const SelectContent = ({ children }: { children: React.ReactNode }) => {
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

  const SelectItem = ({ value, children }: { value: string; children: React.ReactNode }) => (
    <option value={value}>{children}</option>
  )

  const SelectValue = ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>

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
  })

  const defaultSettings: SSHProtocolSettings = {
    session: {
      concurrent_limit: 0,
      idle_timeout_minutes: 0,
      enable_sftp: true,
    },
    terminal: {
      theme_mode: 'auto',
      font_family: 'monospace',
      font_size: 14,
      scrollback_limit: 1000,
    },
    recording: {
      mode: 'optional',
      storage: 'filesystem',
      retention_days: 90,
      require_consent: true,
    },
    collaboration: {
      allow_sharing: true,
      restrict_write_to_admins: false,
    },
  }

  it('allows editing protocol settings fields', async () => {
    hookMock.mockReturnValue({
      data: defaultSettings,
      isLoading: false,
      isFetching: false,
      update: {
        mutateAsync: vi.fn(),
        isPending: false,
        isSuccess: false,
      },
    })

    const user = userEvent.setup()
    render(<ProtocolSettings />)

    const concurrentInput = screen.getByLabelText('Concurrent session limit') as HTMLInputElement
    await user.clear(concurrentInput)
    fireEvent.change(concurrentInput, { target: { value: '5' } })
    expect(concurrentInput.value).toBe('5')

    const idleInput = screen.getByLabelText('Idle timeout (minutes)') as HTMLInputElement
    await user.clear(idleInput)
    fireEvent.change(idleInput, { target: { value: '60' } })
    expect(idleInput.value).toBe('60')

    const themeSelect = screen.getByLabelText('Theme mode') as HTMLSelectElement
    await user.selectOptions(themeSelect, 'force_dark')
    expect(themeSelect.value).toBe('force_dark')

    const fontInput = screen.getByLabelText('Font family') as HTMLInputElement
    await user.clear(fontInput)
    await user.type(fontInput, 'Fira Code')
    expect(fontInput.value).toBe('Fira Code')

    const fontSizeInput = screen.getByLabelText('Font size (px)') as HTMLInputElement
    await user.clear(fontSizeInput)
    fireEvent.change(fontSizeInput, { target: { value: '16' } })
    expect(fontSizeInput.value).toBe('16')

    const scrollbackInput = screen.getByLabelText('Scrollback limit (lines)') as HTMLInputElement
    await user.clear(scrollbackInput)
    fireEvent.change(scrollbackInput, { target: { value: '1500' } })
    expect(scrollbackInput.value).toBe('1500')

    const modeSelect = screen.getByLabelText('Recording mode') as HTMLSelectElement
    await user.selectOptions(modeSelect, 'forced')
    expect(modeSelect.value).toBe('forced')

    const storageSelect = screen.getByLabelText('Recording storage') as HTMLSelectElement
    await user.selectOptions(storageSelect, 's3')
    expect(storageSelect.value).toBe('s3')

    const retentionInput = screen.getByLabelText('Retention (days)') as HTMLInputElement
    await user.clear(retentionInput)
    fireEvent.change(retentionInput, { target: { value: '45' } })
    expect(retentionInput.value).toBe('45')
  })
})
