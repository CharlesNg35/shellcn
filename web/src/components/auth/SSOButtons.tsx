import { forwardRef } from 'react'
import { Building, Globe, KeyRound, ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import type { AuthProviderMetadata } from '@/types/auth'
import { cn } from '@/lib/utils/cn'

interface SSOButtonsProps {
  providers: AuthProviderMetadata[]
  onSelect?: (provider: AuthProviderMetadata) => void
  className?: string
  disabled?: boolean
}

const providerIcons: Record<string, React.ComponentType<{ className?: string }>> = {
  oidc: Globe,
  saml: ShieldCheck,
  ldap: Building,
  local: KeyRound,
}

export const SSOButtons = forwardRef<HTMLDivElement, SSOButtonsProps>(
  ({ providers, onSelect, className, disabled }, ref) => {
    const enabledProviders = providers.filter((provider) => provider.enabled)

    if (enabledProviders.length === 0) {
      return null
    }

    return (
      <div ref={ref} className={cn('grid gap-3 sm:grid-cols-2', className)}>
        {enabledProviders.map((provider) => {
          const Icon = providerIcons[provider.type] ?? Globe

          const content = (
            <>
              <Icon className="h-4 w-4" />
              {provider.name}
            </>
          )

          if (provider.login_url) {
            return (
              <Button
                key={provider.type}
                asChild
                variant="outline"
                disabled={disabled}
                className="justify-center"
              >
                <a href={provider.login_url} data-provider={provider.type}>
                  {content}
                </a>
              </Button>
            )
          }

          return (
            <Button
              key={provider.type}
              type="button"
              variant="outline"
              className="justify-center"
              onClick={() => onSelect?.(provider)}
              disabled={disabled}
            >
              {content}
            </Button>
          )
        })}
      </div>
    )
  }
)

SSOButtons.displayName = 'SSOButtons'
