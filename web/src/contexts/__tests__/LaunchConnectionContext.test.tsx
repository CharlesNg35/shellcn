import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import {
  LaunchConnectionProvider,
  useLaunchConnectionContext,
} from '@/contexts/LaunchConnectionContext'
import * as connectionsApi from '@/lib/api/connections'

vi.mock('@/hooks/useActiveConnections', () => ({
  useActiveConnections: () => ({ data: [], isLoading: false, isError: false, refetch: vi.fn() }),
}))

vi.mock('@/hooks/useConnectionTemplate', () => ({
  useConnectionTemplate: () => ({ data: null, isLoading: false, isError: false, refetch: vi.fn() }),
}))

vi.mock('@/lib/api/active-sessions', () => ({
  launchActiveSession: vi.fn().mockResolvedValue({
    session: {
      id: 'sess-test',
      connection_id: 'conn-123',
      protocol_id: 'ssh',
      user_id: 'user',
      started_at: '2024-01-01T00:00:00Z',
      last_seen_at: '2024-01-01T00:05:00Z',
    },
  }),
}))

const testConnection = {
  id: 'conn-123',
  name: 'QA SSH',
  description: 'QA environment',
  protocol_id: 'ssh',
  team_id: null,
  owner_user_id: 'owner-1',
  folder_id: null,
  metadata: {},
  settings: {},
  identity_id: null,
  last_used_at: null,
  targets: [],
  shares: [],
  share_summary: undefined,
  folder: undefined,
}

function TestConsumer() {
  const launch = useLaunchConnectionContext()
  return (
    <button type="button" onClick={() => launch.openById('conn-123')}>
      Open Launch
    </button>
  )
}

describe('LaunchConnectionProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(connectionsApi, 'fetchConnectionById').mockResolvedValue({
      ...testConnection,
    })
  })

  it('fetches connection details and opens modal via context', async () => {
    const queryClient = new QueryClient()

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <LaunchConnectionProvider>
            <TestConsumer />
          </LaunchConnectionProvider>
        </MemoryRouter>
      </QueryClientProvider>
    )

    fireEvent.click(screen.getByText('Open Launch'))

    await waitFor(() => {
      expect(connectionsApi.fetchConnectionById).toHaveBeenCalledWith('conn-123')
    })

    await waitFor(() => {
      expect(screen.getByText('Launch QA SSH')).toBeInTheDocument()
    })
  })
})
