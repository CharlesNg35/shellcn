import { useEffect } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import logo from '@/assets/logo.svg'
import { useAuth } from '@/hooks/useAuth'
import { SSOButtons } from '@/components/auth/SSOButtons'
import { ThemeToggle } from '@/components/theme/ThemeToggle'
import { Terminal, Shield, Activity, Lock } from 'lucide-react'
import { APP_NAME, APP_DESCRIPTION } from '@/lib/constants'

export function AuthLayout() {
  const { providers, loadProviders } = useAuth()
  const location = useLocation()

  useEffect(() => {
    if (!providers.length) {
      void loadProviders().catch(() => {
        // Swallow errors; provider list is optional for login
      })
    }
  }, [providers.length, loadProviders])

  return (
    <div className="flex min-h-screen bg-background">
      {/* Left side - Branding */}
      <aside className="relative hidden w-1/2 overflow-hidden border-r border-border bg-card lg:flex lg:flex-col">
        <div className="flex flex-1 flex-col justify-between p-12">
          {/* Logo and title */}
          <div className="space-y-6">
            <Link to="/" className="flex items-center gap-3">
              <img src={logo} alt={APP_NAME} className="h-12 w-12" />
              <div>
                <h1 className="text-2xl font-bold text-foreground">{APP_NAME}</h1>
                <p className="text-sm text-muted-foreground">{APP_DESCRIPTION}</p>
              </div>
            </Link>

            <div className="max-w-md space-y-4 pt-8">
              <h2 className="text-3xl font-bold text-foreground">Secure Infrastructure Access</h2>
              <p className="text-base text-muted-foreground">
                Centralized gateway for managing enterprise infrastructure access. Connect to SSH
                servers, Docker hosts, Kubernetes clusters, and databases.
              </p>
            </div>
          </div>

          {/* Features list */}
          <div className="space-y-4">
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                <Terminal className="h-5 w-5 text-primary" />
              </div>
              <div>
                <h3 className="font-medium text-foreground">Multi-Protocol Access</h3>
                <p className="text-sm text-muted-foreground">SSH, Telnet, RDP, VNC, and more</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-accent/10">
                <Shield className="h-5 w-5 text-accent" />
              </div>
              <div>
                <h3 className="font-medium text-foreground">Enterprise Authentication</h3>
                <p className="text-sm text-muted-foreground">OIDC, SAML, LDAP, and local auth</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-secondary/10">
                <Activity className="h-5 w-5 text-secondary" />
              </div>
              <div>
                <h3 className="font-medium text-foreground">Session Recording</h3>
                <p className="text-sm text-muted-foreground">Full audit trail and compliance</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-chart-3/10">
                <Lock className="h-5 w-5 text-chart-3" />
              </div>
              <div>
                <h3 className="font-medium text-foreground">Fine-Grained Permissions</h3>
                <p className="text-sm text-muted-foreground">Role-based access control</p>
              </div>
            </div>
          </div>
        </div>
      </aside>

      {/* Right side - Auth form */}
      <main className="flex flex-1 flex-col justify-center px-6 py-12 sm:px-12 lg:px-16">
        <div className="mx-auto w-full max-w-md">
          {/* Theme toggle - top right */}
          <div className="mb-6 flex justify-end">
            <ThemeToggle />
          </div>

          {/* Mobile logo */}
          <div className="mb-10 flex items-center gap-3 lg:hidden">
            <img src={logo} alt={APP_NAME} className="h-10 w-10" />
            <div>
              <h1 className="text-xl font-bold text-foreground">{APP_NAME}</h1>
              <p className="text-sm text-muted-foreground">{APP_DESCRIPTION}</p>
            </div>
          </div>

          {/* Auth form */}
          <div className="space-y-6">
            <Outlet />

            {/* SSO providers */}
            {providers.length > 0 && !location.pathname.includes('/setup') && (
              <div className="space-y-4">
                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <div className="w-full border-t border-border" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-background px-2 text-muted-foreground">
                      Or continue with
                    </span>
                  </div>
                </div>
                <SSOButtons providers={providers} />
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
