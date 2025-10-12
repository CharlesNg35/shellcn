import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'

export function SetupGuard() {
  const { fetchSetupStatus, setupStatus, isSetupStatusLoading } = useAuth({ autoInitialize: false })
  const location = useLocation()
  const [localStatus, setLocalStatus] = useState<'pending' | 'complete' | 'loading'>(() =>
    setupStatus ? setupStatus.status : 'loading'
  )

  useEffect(() => {
    let active = true

    if (setupStatus) {
      setLocalStatus(setupStatus.status)
      return () => {
        active = false
      }
    }

    if (isSetupStatusLoading) {
      setLocalStatus('loading')
      return () => {
        active = false
      }
    }

    setLocalStatus('loading')
    fetchSetupStatus()
      .then((status) => {
        if (active) {
          setLocalStatus(status.status)
        }
      })
      .catch(() => {
        if (active) {
          setLocalStatus('complete')
        }
      })

    return () => {
      active = false
    }
  }, [setupStatus, isSetupStatusLoading, fetchSetupStatus])

  if (localStatus === 'loading') {
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
  if (localStatus === 'pending' && location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />
  }

  // If setup is complete and we're on the setup page, redirect to login
  if (localStatus === 'complete' && location.pathname === '/setup') {
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}
