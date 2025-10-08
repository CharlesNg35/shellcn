import { useEffect } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import logo from '@/assets/logo.svg'
import { useAuth } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { SSOButtons } from '@/components/auth/SSOButtons'

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
    <div className="grid min-h-screen bg-background md:grid-cols-[1fr,1fr]">
      <aside className="relative hidden overflow-hidden bg-gradient-to-br from-primary via-primary/80 to-primary/60 text-primary-foreground md:flex md:flex-col">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(255,255,255,0.08),rgba(255,255,255,0))]" />
        <div className="relative z-10 flex flex-1 flex-col justify-between px-12 py-10">
          <div className="space-y-6">
            <Link to="/" className="flex items-center gap-3 text-lg font-semibold">
              <img src={logo} alt="ShellCN" className="h-10 w-10" />
              ShellCN Platform
            </Link>
            <p className="max-w-sm text-sm text-primary-foreground/80">
              Secure enterprise access to your infrastructure. Authenticate once, access every
              protocol with audit-grade visibility.
            </p>
          </div>

          <div className="space-y-6">
            <div>
              <p className="text-sm font-medium uppercase tracking-wide text-primary-foreground/70">
                Platform Highlights
              </p>
              <ul className="mt-4 space-y-3 text-sm text-primary-foreground/80">
                <li>• Multi-protocol access across SSH, RDP, VNC, Kubernetes, and more</li>
                <li>• Fine-grained roles, permissions, and auditing</li>
                <li>• Built-in session recording and MFA enforcement</li>
              </ul>
            </div>
            <div className="rounded-lg border border-white/10 bg-white/5 p-6 backdrop-blur">
              <h3 className="text-lg font-semibold">Need an account?</h3>
              <p className="mt-2 text-sm text-primary-foreground/80">
                Initial setup is handled by your system administrator. Contact support if you need
                assistance.
              </p>
              <Button asChild variant="secondary" className="mt-4 w-full bg-white text-primary">
                <a href="mailto:support@shellcn.io">Contact Support</a>
              </Button>
            </div>
          </div>
        </div>
      </aside>

      <main className="flex flex-col justify-center px-6 py-8 sm:px-8 md:px-12">
        <div className="mx-auto w-full max-w-md">
          <div className="mb-8 flex items-center gap-3 md:hidden">
            <img src={logo} alt="ShellCN" className="h-12 w-12" />
            <div>
              <h1 className="text-2xl font-semibold text-foreground">ShellCN Platform</h1>
              <p className="text-sm text-muted-foreground">Enterprise remote access management</p>
            </div>
          </div>

          <div className="rounded-2xl border border-border bg-card p-8 shadow-lg shadow-black/5 sm:p-10">
            <Outlet />

            {providers.length > 0 && !location.pathname.includes('/setup') && (
              <div className="mt-8">
                <div className="relative flex items-center justify-center text-xs uppercase text-muted-foreground">
                  <span className="bg-card px-2">or continue with</span>
                  <span className="absolute inset-x-0 h-px bg-border" />
                </div>
                <div className="mt-6">
                  <SSOButtons providers={providers} />
                </div>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
