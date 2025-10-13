import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { Sidebar } from './Sidebar'
import { Header } from './Header'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/hooks/useAuth'
import { resendEmailVerification } from '@/lib/api/profile'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'

export function DashboardLayout() {
  const [isSidebarOpen, setSidebarOpen] = useState(false)
  const location = useLocation()
  const { user } = useAuth()

  const needsEmailVerification = Boolean(
    user && (user.auth_provider ?? 'local') === 'local' && user.email_verified === false
  )

  const resendMutation = useMutation({
    mutationFn: () => resendEmailVerification(),
    onSuccess: () => {
      toast.success('Verification email sent', {
        description: 'Check your inbox to confirm your email address.',
      })
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Unable to resend verification email', {
        description: apiError.message,
      })
    },
  })

  useEffect(() => {
    setSidebarOpen(false)
  }, [location.pathname])

  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar isOpen={isSidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="flex flex-1 flex-col lg:pl-64">
        <Header onToggleSidebar={() => setSidebarOpen(true)} />
        <main className="flex flex-1 flex-col bg-muted/20">
          <div className="flex flex-1 flex-col gap-4 p-4 lg:gap-6 lg:p-6">
            {needsEmailVerification ? (
              <div className="flex flex-col gap-3 rounded-lg border border-amber-400/60 bg-amber-100/70 p-4 text-amber-900 shadow-sm">
                <div>
                  <p className="text-sm font-semibold">Verify your email address</p>
                  <p className="text-sm text-amber-800/90">
                    We sent a verification link to {user?.email}. Confirm your email to unlock all
                    features.
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => resendMutation.mutate()}
                    loading={resendMutation.isPending}
                  >
                    Resend verification email
                  </Button>
                </div>
              </div>
            ) : null}
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
