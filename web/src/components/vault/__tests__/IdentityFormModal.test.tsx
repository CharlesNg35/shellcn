import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { IdentityFormModal } from '@/components/vault/IdentityFormModal'
import type { CredentialTemplateRecord, IdentityRecord } from '@/types/vault'

const MASKED_SECRET = '••••••••'

const mockUseCredentialTemplates = vi.fn()
const mockUseIdentity = vi.fn()
const mockUseIdentityMutations = vi.fn()
const mockUseTeams = vi.fn()

vi.mock('@/hooks/useIdentities', () => ({
  useCredentialTemplates: (options: unknown) => mockUseCredentialTemplates(options),
  useIdentity: (identityId: string | undefined, options: unknown) =>
    mockUseIdentity(identityId, options),
  useIdentityMutations: (identityId?: string) => mockUseIdentityMutations(identityId),
}))

vi.mock('@/hooks/useTeams', () => ({
  useTeams: (options: unknown) => mockUseTeams(options),
}))

const { toastWarning } = vi.hoisted(() => ({
  toastWarning: vi.fn(),
}))

vi.mock('@/lib/utils/toast', () => ({
  toast: {
    warning: toastWarning,
    dismiss: vi.fn(),
  },
}))

const template: CredentialTemplateRecord = {
  id: 'tpl-ssh',
  driver_id: 'ssh',
  version: '1.0.0',
  display_name: 'SSH',
  description: null,
  fields: [
    {
      name: 'private_key',
      type: 'secret',
      label: 'Private Key',
      description: 'SSH private key',
      input_modes: ['textarea'],
      required: true,
    },
    {
      name: 'username',
      type: 'string',
      label: 'Username',
      required: true,
    },
  ],
  compatible_protocols: ['ssh'],
  deprecated_after: null,
  metadata: null,
  hash: 'hash',
}

const identity: IdentityRecord = {
  id: 'identity-1',
  name: 'Production SSH',
  description: 'Root access',
  scope: 'global',
  owner_user_id: 'usr-1',
  team_id: null,
  connection_id: null,
  template_id: template.id,
  version: 1,
  metadata: {},
  usage_count: 5,
  last_used_at: null,
  last_rotated_at: null,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-02T00:00:00Z',
  payload: {
    private_key: '-----BEGIN KEY-----',
    username: 'root',
  },
  shares: [],
  connection_count: 0,
}

function arrange() {
  mockUseCredentialTemplates.mockReturnValue({
    data: [template],
    isLoading: false,
  })
  mockUseIdentity.mockReturnValue({
    data: identity,
    isLoading: false,
  })
  mockUseIdentityMutations.mockReturnValue({
    create: { mutateAsync: vi.fn(), isPending: false },
    update: { mutateAsync: vi.fn(), isPending: false },
    remove: { mutateAsync: vi.fn(), isPending: false },
  })
  mockUseTeams.mockReturnValue({
    data: { data: [] },
    isLoading: false,
  })
  toastWarning.mockReset()
}

describe('IdentityFormModal', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    arrange()
  })

  it('masks secret values when editing an identity', () => {
    render(
      <IdentityFormModal
        open
        mode="edit"
        identityId={identity.id}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const secretField = screen.getByLabelText(/Private Key/i)
    expect(secretField).toHaveValue(MASKED_SECRET)
    expect(secretField).toBeDisabled()
    const hint = screen.getByText(/Hidden for security/i)
    expect(hint).toBeInTheDocument()
  })

  it('clears and enables secret fields when rotating credentials', () => {
    render(
      <IdentityFormModal
        open
        mode="edit"
        identityId={identity.id}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const rotateCheckbox = screen.getByLabelText(/Rotate credentials/i)
    fireEvent.click(rotateCheckbox)

    const secretField = screen.getByLabelText(/Private Key/i)
    expect(secretField).not.toBeDisabled()
    expect(secretField).toHaveValue('')
  })

  it('blocks clipboard copy operations for secret inputs', () => {
    render(
      <IdentityFormModal
        open
        mode="edit"
        identityId={identity.id}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const rotateCheckbox = screen.getByLabelText(/Rotate credentials/i)
    fireEvent.click(rotateCheckbox)

    const secretField = screen.getByLabelText(/Private Key/i)
    fireEvent.change(secretField, { target: { value: 'new-secret-value' } })
    fireEvent.copy(secretField)

    expect(toastWarning).toHaveBeenCalledWith('Copy disabled', {
      description: 'Secret values cannot be copied from the vault form.',
    })
  })
})
