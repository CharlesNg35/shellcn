import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { Key } from 'lucide-react'
import { ProviderCard } from '@/components/auth-providers/ProviderCard'
import type { AuthProviderRecord } from '@/types/auth-providers'

const baseProvider: AuthProviderRecord = {
  id: 'oidc',
  type: 'oidc',
  name: 'OIDC',
  enabled: false,
  allowRegistration: false,
  requireEmailVerification: true,
  allowPasswordReset: false,
}

describe('ProviderCard', () => {
  it('renders not configured state', () => {
    render(
      <ProviderCard
        type="oidc"
        name="OpenID Connect"
        description="OIDC provider."
        icon={Key}
        onConfigure={() => {}}
        toggleDisabled
      />
    )

    expect(screen.getByText(/Not Configured/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /configure/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /enable/i })).not.toBeInTheDocument()
  })

  it('invokes toggle callback when enabled', () => {
    const handleToggle = vi.fn()
    render(
      <ProviderCard
        type="oidc"
        name="OpenID Connect"
        description="OIDC provider."
        icon={Key}
        provider={baseProvider}
        onConfigure={() => {}}
        onToggleEnabled={handleToggle}
      />
    )

    const enableButton = screen.getByRole('button', { name: /enable/i })
    fireEvent.click(enableButton)

    expect(handleToggle).toHaveBeenCalledWith(true)
  })

  it('displays provider metadata', () => {
    render(
      <ProviderCard
        type="local"
        name="Local"
        description="Local provider."
        icon={Key}
        provider={{
          ...baseProvider,
          id: 'local',
          type: 'local',
          enabled: true,
          allowRegistration: true,
          allowPasswordReset: true,
        }}
        onConfigure={() => {}}
      />
    )

    expect(screen.getByText(/Self-registration/i)).toBeInTheDocument()
    expect(screen.getByText(/Password reset/i)).toBeInTheDocument()
  })
})
