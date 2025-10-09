import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as usePermissionsModule from '@/hooks/usePermissions'
import * as useConnectionFoldersModule from '@/hooks/useConnectionFolders'
import { Sidebar } from '../Sidebar'

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: vi.fn(),
}))

vi.mock('@/hooks/useConnectionFolders', () => ({
  useConnectionFolders: vi.fn(),
}))

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

    vi.spyOn(useConnectionFoldersModule, 'useConnectionFolders').mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useConnectionFoldersModule.useConnectionFolders>)

    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    // Sidebar renders twice (desktop + mobile), so we use getAllByText
    expect(screen.getAllByText(/Dashboard/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Connections/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Settings/i).length).toBeGreaterThan(0)
  })
})
