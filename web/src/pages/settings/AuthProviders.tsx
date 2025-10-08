import { Key, Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function AuthProviders() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Authentication Providers</h1>
          <p className="mt-2 text-muted-foreground">
            Configure OIDC, SAML, LDAP, and other authentication providers
          </p>
        </div>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Add Provider
        </Button>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Key className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">
          Authentication Provider Management
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Authentication provider configuration will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
