import { Navigate, useNavigate } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useAuth } from '@/hooks/useAuth'
import { MfaVerificationForm } from '@/components/auth/MfaVerificationForm'
import { Button } from '@/components/ui/Button'

export function MfaVerification() {
  const { mfaChallenge, logout } = useAuth()
  const navigate = useNavigate()

  if (!mfaChallenge) {
    return <Navigate to="/login" replace />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col items-center gap-3 text-center">
        <span className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
          <ShieldCheck className="h-6 w-6" />
        </span>
        <div className="space-y-1">
          <h2 className="text-2xl font-semibold">Verify it&apos;s you</h2>
          <p className="text-sm text-muted-foreground">
            Complete the {mfaChallenge.method?.toUpperCase()} challenge to continue.
          </p>
        </div>
      </div>

      <MfaVerificationForm
        onSuccess={() => navigate('/dashboard', { replace: true })}
        onFailure={() => navigate('/login?notice=mfa_failed', { replace: true })}
      />

      <div className="text-center text-sm text-muted-foreground">
        Having trouble?{' '}
        <Button variant="link" type="button" onClick={() => logout()}>
          Return to login
        </Button>
      </div>
    </div>
  )
}
