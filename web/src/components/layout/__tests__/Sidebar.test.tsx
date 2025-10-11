import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as usePermissionsModule from '@/hooks/usePermissions'
import * as useActiveConnectionsModule from '@/hooks/useActiveConnections'
import { Sidebar } from '../Sidebar'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: vi.fn(),
}))

vi.mock('@/hooks/useActiveConnections', () => ({
  useActiveConnections: vi.fn(),
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

    vi.spyOn(useActiveConnectionsModule, 'useActiveConnections').mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useActiveConnectionsModule.useActiveConnections>)

    const queryClient = new QueryClient()

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Sidebar />
        </MemoryRouter>
      </QueryClientProvider>
    )

    // Sidebar renders twice (desktop + mobile), so we use getAllByText
    expect(screen.getAllByText(/Dashboard/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Connections/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Settings/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Active Sessions/i).length).toBeGreaterThan(0)
  })
})
