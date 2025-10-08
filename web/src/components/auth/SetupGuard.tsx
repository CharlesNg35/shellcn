import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'

export function SetupGuard() {
  const { fetchSetupStatus } = useAuth({ autoInitialize: false })
  const location = useLocation()
  const [setupStatus, setSetupStatus] = useState<'pending' | 'complete' | 'loading'>('loading')

  useEffect(() => {
    let active = true

    fetchSetupStatus()
      .then((status) => {
        if (active) {
          setSetupStatus(status.status)
        }
      })
      .catch(() => {
        if (active) {
          setSetupStatus('complete') // Assume complete on error
        }
      })

    return () => {
      active = false
    }
  }, [fetchSetupStatus])

  if (setupStatus === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-3">
          <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
          <p className="text-sm text-muted-foreground">Loading...</p>
        </div>
      </div>
    )
  }

  // If setup is pending and we're not on the setup page, redirect to setup
  if (setupStatus === 'pending' && location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />
  }

  // If setup is complete and we're on the setup page, redirect to login
  if (setupStatus === 'complete' && location.pathname === '/setup') {
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}
