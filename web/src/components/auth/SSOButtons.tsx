import { forwardRef, type ComponentType } from 'react'
import { Building, Globe, KeyRound, ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import type { AuthProviderMetadata } from '@/types/auth'
import { cn } from '@/lib/utils/cn'

interface SSOButtonsProps {
  providers: AuthProviderMetadata[]
  onSelect?: (provider: AuthProviderMetadata) => void
  className?: string
  disabled?: boolean
  successRedirect?: string
  errorRedirect?: string
}

const providerIcons: Record<string, ComponentType<{ className?: string }>> = {
  oidc: Globe,
  saml: ShieldCheck,
  ldap: Building,
  local: KeyRound,
}

export const SSOButtons = forwardRef<HTMLDivElement, SSOButtonsProps>(
  ({ providers, onSelect, className, disabled, successRedirect, errorRedirect }, ref) => {
    const enabledProviders = providers.filter(
      (provider) => provider.enabled && (provider.flow ?? 'password') === 'redirect'
    )

    if (enabledProviders.length === 0) {
      return null
    }

    return (
      <div ref={ref} className={cn('grid gap-3 sm:grid-cols-2', className)}>
        {enabledProviders.map((provider) => {
          const Icon = providerIcons[provider.type] ?? Globe
          const redirect = encodeURIComponent(successRedirect ?? '/dashboard')
          const errorTarget = encodeURIComponent(errorRedirect ?? '/login?error=sso_failed')
          const loginHref =
            provider.login_url ??
            `/api/auth/providers/${provider.type}/login?redirect=${redirect}&error_redirect=${errorTarget}`

          const content = (
            <>
              <Icon className="h-4 w-4" />
              {provider.name}
            </>
          )

          return (
            <Button
              key={provider.type}
              asChild
              variant="outline"
              disabled={disabled}
              className="justify-center"
            >
              <a
                href={loginHref}
                data-provider={provider.type}
                onClick={() => onSelect?.(provider)}
              >
                {content}
              </a>
            </Button>
          )
        })}
      </div>
    )
  }
)

SSOButtons.displayName = 'SSOButtons'
