import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { IdentitySelector } from '@/components/vault/IdentitySelector'
import type { IdentityRecord } from '@/types/vault'

const sampleIdentities: IdentityRecord[] = [
  {
    id: 'id-1',
    name: 'Global SSH',
    description: 'Shared root access',
    scope: 'global',
    owner_user_id: 'usr-1',
    version: 1,
    metadata: {},
    usage_count: 3,
    connection_count: 2,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-02T00:00:00Z',
    team_id: null,
    connection_id: null,
    template_id: null,
    payload: undefined,
    shares: [],
    last_used_at: null,
    last_rotated_at: null,
  },
  {
    id: 'id-2',
    name: 'Team DB Credential',
    description: 'Database access for analytics',
    scope: 'team',
    owner_user_id: 'usr-2',
    version: 1,
    metadata: {},
    usage_count: 10,
    connection_count: 4,
    created_at: '2024-01-05T00:00:00Z',
    updated_at: '2024-01-05T00:00:00Z',
    team_id: 'team-1',
    connection_id: null,
    template_id: null,
    payload: undefined,
    shares: [],
    last_used_at: null,
    last_rotated_at: null,
  },
]

const mockUseIdentities = vi.fn()

vi.mock('@/hooks/useIdentities', () => ({
  useIdentities: (params: unknown, options: unknown) => mockUseIdentities(params, options),
}))

describe('IdentitySelector', () => {
  beforeEach(() => {
    mockUseIdentities.mockReset()
    mockUseIdentities.mockReturnValue({ data: sampleIdentities, isLoading: false })
  })

  it('lists identities and triggers selection', () => {
    const handleChange = vi.fn()
    render(<IdentitySelector value={null} onChange={handleChange} />)

    const option = screen.getByRole('button', { name: /Global SSH/i })
    fireEvent.click(option)
    expect(handleChange).toHaveBeenCalledWith('id-1')
  })

  it('renders inline create button when allowed', () => {
    const handleCreate = vi.fn()
    render(
      <IdentitySelector
        value={null}
        onChange={vi.fn()}
        allowInlineCreate
        onCreateIdentity={handleCreate}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: /Create new identity/i }))
    expect(handleCreate).toHaveBeenCalled()
  })
})
