import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as usePermissionsModule from '@/hooks/usePermissions'
import * as useConnectionSummaryModule from '@/hooks/useConnectionSummary'
import * as useProtocolsModule from '@/hooks/useProtocols'
import { Sidebar } from '../Sidebar'

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: vi.fn(),
}))

vi.mock('@/hooks/useConnectionSummary', () => ({
  useConnectionSummary: vi.fn(),
}))

vi.mock('@/hooks/useProtocols', () => ({
  useAvailableProtocols: vi.fn(),
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

    vi.spyOn(useConnectionSummaryModule, 'useConnectionSummary').mockReturnValue({
      data: [
        { protocol_id: 'ssh', count: 5 },
        { protocol_id: 'rdp', count: 2 },
      ],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useConnectionSummaryModule.useConnectionSummary>)

    vi.spyOn(useProtocolsModule, 'useAvailableProtocols').mockReturnValue({
      data: {
        data: [
          { id: 'ssh', name: 'SSH', description: '', features: [], icon: 'ssh' },
          { id: 'rdp', name: 'RDP', description: '', features: [], icon: 'rdp' },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProtocolsModule.useAvailableProtocols>)

    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    // Sidebar renders twice (desktop + mobile), so we use getAllByText
    expect(screen.getAllByText(/Dashboard/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Connections/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Settings/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Protocols/i).length).toBeGreaterThan(0)
  })
})
