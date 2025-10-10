import { render, screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi } from 'vitest'
import { TeamConnectionsPanel } from './TeamConnectionsPanel'
import * as useConnectionFoldersModule from '@/hooks/useConnectionFolders'
import * as useConnectionSummaryModule from '@/hooks/useConnectionSummary'
import * as useConnectionsModule from '@/hooks/useConnections'
import * as useProtocolsModule from '@/hooks/useProtocols'

vi.mock('@/hooks/useConnectionFolders', () => ({
  useConnectionFolders: vi.fn(),
}))

vi.mock('@/hooks/useConnectionSummary', () => ({
  useConnectionSummary: vi.fn(),
}))

vi.mock('@/hooks/useConnections', () => ({
  useConnections: vi.fn(),
}))

vi.mock('@/hooks/useProtocols', () => ({
  useAvailableProtocols: vi.fn(),
}))

describe('TeamConnectionsPanel', () => {
  it('renders empty state when no connections', () => {
    vi.spyOn(useConnectionFoldersModule, 'useConnectionFolders').mockReturnValue({
      data: [],
      isLoading: false,
    } as never)

    vi.spyOn(useConnectionSummaryModule, 'useConnectionSummary').mockReturnValue({
      data: [],
      isLoading: false,
    } as never)

    vi.spyOn(useConnectionsModule, 'useConnections').mockReturnValue({
      data: { data: [] },
      isLoading: false,
    } as never)

    vi.spyOn(useProtocolsModule, 'useAvailableProtocols').mockReturnValue({
      data: { data: [] },
      isLoading: false,
    } as never)

    const router = createMemoryRouter([
      {
        path: '/',
        element: <TeamConnectionsPanel teamId="team-1" />,
      },
    ])

    render(<RouterProvider router={router} />)

    expect(screen.getByText(/No connections found/i)).toBeInTheDocument()
    expect(screen.getByText(/No connections available yet/i)).toBeInTheDocument()
  })
})
