import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SSHPreferencesPanel } from '../SSHPreferencesPanel'
import type { UserPreferences } from '@/types/preferences'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { toast } from 'sonner'

const hookMock = vi.fn()
vi.mock('@/hooks/useUserPreferences', () => ({
  useUserPreferences: () => hookMock(),
}))

vi.mock('../PersonalSnippetsSection', () => ({
  PersonalSnippetsSection: () => <div data-testid="personal-snippets-section" />,
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

describe('SSHPreferencesPanel', () => {
  const defaultPreferences: UserPreferences = {
    ssh: {
      terminal: {
        font_family: 'Fira Code',
        cursor_style: 'block',
        copy_on_select: true,
        font_size: 14,
        scrollback_limit: 1000,
      },
      sftp: {
        show_hidden_files: false,
        auto_open_queue: true,
        confirm_before_overwrite: true,
      },
    },
  }

  beforeEach(() => {
    hookMock.mockReset()
    vi.mocked(toast.success).mockReset()
    vi.mocked(toast.error).mockReset()
  })

  it('submits updated preferences', async () => {
    const mutateAsync = vi.fn().mockResolvedValue(defaultPreferences)
    hookMock.mockReturnValue({
      data: defaultPreferences,
      isLoading: false,
      update: {
        mutateAsync,
        isPending: false,
        isSuccess: false,
      },
    })

    const user = userEvent.setup()
    render(<SSHPreferencesPanel />)

    const fontInput = screen.getByLabelText('Font family') as HTMLInputElement
    await user.clear(fontInput)
    await user.type(fontInput, 'JetBrains Mono')

    const cursorSelect = screen.getByLabelText('Cursor style') as HTMLSelectElement
    await user.selectOptions(cursorSelect, 'beam')

    const fontSizeInput = screen.getByLabelText('Font size (px)') as HTMLInputElement
    fireEvent.change(fontSizeInput, { target: { value: '16' } })

    const scrollbackInput = screen.getByLabelText('Scrollback limit') as HTMLInputElement
    fireEvent.change(scrollbackInput, { target: { value: '2000' } })

    await user.click(screen.getByLabelText('Copy on select'))
    await user.click(screen.getByLabelText('Show hidden files by default'))
    await user.click(screen.getByLabelText('Open transfer queue automatically'))
    await user.click(screen.getByLabelText('Confirm before overwriting files'))

    await user.click(screen.getByRole('button', { name: /save preferences/i }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        ssh: {
          terminal: {
            font_family: 'JetBrains Mono',
            cursor_style: 'beam',
            copy_on_select: false,
            font_size: 16,
            scrollback_limit: 2000,
          },
          sftp: {
            show_hidden_files: true,
            auto_open_queue: false,
            confirm_before_overwrite: false,
          },
        },
      })
    )

    expect(toast.success).toHaveBeenCalledWith('Preferences updated', {
      description: 'Your SSH defaults were saved successfully.',
    })
  })

  it('handles update errors', async () => {
    const mutateAsync = vi.fn().mockRejectedValue(new Error('server error'))
    hookMock.mockReturnValue({
      data: defaultPreferences,
      isLoading: false,
      update: {
        mutateAsync,
        isPending: false,
        isSuccess: false,
      },
    })

    const user = userEvent.setup()
    render(<SSHPreferencesPanel />)

    await user.click(screen.getByRole('button', { name: /save preferences/i }))

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled())
    expect(toast.error).toHaveBeenCalledWith('Unable to update preferences', {
      description: 'server error',
    })
  })
})
