import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as usePermissionsModule from '@/hooks/usePermissions'
import { Sidebar } from '../Sidebar'

vi.mock('@/hooks/usePermissions')

describe('Sidebar', () => {
  it('renders navigation items', () => {
    vi.spyOn(usePermissionsModule, 'usePermissions').mockReturnValue({
      hasPermission: () => true,
      hasAnyPermission: () => true,
      hasAllPermissions: () => true,
      permissions: [],
      isLoading: false,
      refetch: async () => {},
    } as unknown as ReturnType<typeof usePermissionsModule.usePermissions>)

    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    expect(screen.getByText(/Dashboard/i)).toBeInTheDocument()
    expect(screen.getByText(/Connections/i)).toBeInTheDocument()
    expect(screen.getByText(/Settings/i)).toBeInTheDocument()
  })
})
