import { useState } from 'react'
import { describe, expect, it } from 'vitest'
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
    display_name: 'View Users',
  },
  'user.update': {
    id: 'user.update',
    module: 'core',
    description: 'Update users',
    depends_on: ['user.view'],
    implies: [],
    display_name: 'Update Users',
  },
  'user.delete': {
    id: 'user.delete',
    module: 'core',
    description: 'Delete users',
    depends_on: ['user.view', 'user.update'],
    implies: [],
    display_name: 'Delete Users',
  },
}

describe('<PermissionMatrix />', () => {
  function renderWithState(initialSelected: string[] = []) {
    const Wrapper = () => {
      const [selected, setSelected] = useState<Set<string>>(new Set(initialSelected))
      const handleChange = (next: string[]) => {
        setSelected(new Set(next))
      }

      return (
        <PermissionMatrix
          registry={registry}
          selected={selected}
          onChange={handleChange}
          disabled={false}
        />
      )
    }

    render(<Wrapper />)
  }

  it('automatically selects dependencies when enabling a permission', async () => {
    const user = userEvent.setup()
    renderWithState()

    const moduleButton = screen.getByRole('button', { name: /Core Platform/i })
    await user.click(moduleButton)

    const userNamespaceButton = screen.getByRole('button', { name: /^User/i })
    await user.click(userNamespaceButton)

    const deleteCheckbox = screen.getByRole('checkbox', { name: /Delete Users/i })
    await user.click(deleteCheckbox)

    const updateCheckbox = screen.getByRole('checkbox', { name: /Update Users/i })
    const viewCheckbox = screen.getByRole('checkbox', { name: /View Users/i })

    expect(deleteCheckbox).toBeChecked()
    expect(updateCheckbox).toBeChecked()
    expect(viewCheckbox).toBeChecked()
  })

  it('deselects dependent permissions when a dependency is removed', async () => {
    const user = userEvent.setup()
    renderWithState(['user.view', 'user.update', 'user.delete'])

    const moduleButton = screen.getByRole('button', { name: /Core Platform/i })
    await user.click(moduleButton)

    const userNamespaceButton = screen.getByRole('button', { name: /^User/i })
    await user.click(userNamespaceButton)

    const updateCheckbox = screen.getByRole('checkbox', { name: /Update Users/i })
    await user.click(updateCheckbox)

    const deleteCheckbox = screen.getByRole('checkbox', { name: /Delete Users/i })
    expect(updateCheckbox).not.toBeChecked()
    expect(deleteCheckbox).not.toBeChecked()
  })
})
