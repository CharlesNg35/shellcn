import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PermissionMatrix } from '@/components/permissions/PermissionMatrix'
import type { PermissionRegistry } from '@/types/permission'

const registry: PermissionRegistry = {
  'user.view': {
    id: 'user.view',
    module: 'core',
    description: 'View users',
    depends_on: [],
    implies: [],
  },
  'user.edit': {
    id: 'user.edit',
    module: 'core',
    description: 'Edit users',
    depends_on: ['user.view'],
    implies: [],
  },
  'user.delete': {
    id: 'user.delete',
    module: 'core',
    description: 'Delete users',
    depends_on: ['user.view', 'user.edit'],
    implies: [],
  },
}

describe('<PermissionMatrix />', () => {
  it('expands dependencies when enabling a permission', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()

    render(<PermissionMatrix registry={registry} selected={new Set()} onChange={onChange} />)

    // First, expand the module to reveal permissions
    const moduleButton = screen.getByRole('button', { name: /Core Platform/i })
    await user.click(moduleButton)

    // Then expand the "user" namespace
    const userNamespaceButton = screen.getByRole('button', { name: /^User/i })
    await user.click(userNamespaceButton)

    // Now we can access the checkbox
    const deleteCheckbox = screen.getByRole('checkbox', { name: 'user.delete' })
    await user.click(deleteCheckbox)

    expect(onChange).toHaveBeenCalledTimes(1)
    const nextSelection = onChange.mock.calls[0][0] as string[]
    expect(nextSelection.sort()).toEqual(['user.delete', 'user.edit', 'user.view'].sort())
  })

  it('locks dependencies when they are required by selected permissions', async () => {
    const user = userEvent.setup()
    const selected = new Set(['user.view', 'user.edit', 'user.delete'])
    render(<PermissionMatrix registry={registry} selected={selected} onChange={vi.fn()} />)

    // Expand the module to reveal permissions
    const moduleButton = screen.getByRole('button', { name: /Core Platform/i })
    await user.click(moduleButton)

    // Then expand the "user" namespace
    const userNamespaceButton = screen.getByRole('button', { name: /^User/i })
    await user.click(userNamespaceButton)

    // Now check if the checkbox is disabled
    const editCheckbox = screen.getByRole('checkbox', { name: 'user.edit' })
    expect(editCheckbox).toBeDisabled()
  })
})
