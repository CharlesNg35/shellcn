import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { passwordResetConfirmSchema } from '@/schemas/auth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/hooks/useAuth'

type PasswordResetConfirmFormData = z.infer<typeof passwordResetConfirmSchema>

export function PasswordResetConfirm() {
  const navigate = useNavigate()
  const { confirmPasswordReset, providers, loadProviders } = useAuth({ autoInitialize: false })
  const [params] = useSearchParams()
  const prefilledToken = useMemo(() => params.get('token') ?? '', [params])
  const [error, setError] = useState<string | null>(null)
  const [completed, setCompleted] = useState(false)
  const [providersReady, setProvidersReady] = useState(providers.length > 0)

  useEffect(() => {
    let active = true
    if (providers.length === 0 && !providersReady) {
      loadProviders()
        .catch(() => undefined)
        .finally(() => {
          if (active) {
            setProvidersReady(true)
          }
        })
      return () => {
        active = false
      }
    }

    if (providers.length > 0 && !providersReady) {
      setProvidersReady(true)
    }

    return () => {
      active = false
    }
  }, [providers.length, providersReady, loadProviders])

  const canResetPassword = useMemo(() => {
    const localProvider = providers.find((provider) => provider.type === 'local')
    if (!localProvider) {
      return false
    }
    const flag = localProvider.allow_password_reset
    return flag === undefined ? true : Boolean(flag)
  }, [providers])

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<PasswordResetConfirmFormData>({
    resolver: zodResolver(passwordResetConfirmSchema),
    defaultValues: {
      token: prefilledToken,
      password: '',
      confirmPassword: '',
    },
  })

  const onSubmit = async (data: PasswordResetConfirmFormData) => {
    setError(null)

    try {
      await confirmPasswordReset({
        token: data.token,
        password: data.password,
      })
      setCompleted(true)
    } catch (submitError) {
      if (submitError instanceof Error) {
        setError(submitError.message)
      } else {
        setError('Unable to reset password. Please try again or request a new link.')
      }
    }
  }

  if (!providersReady) {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">Loading reset flow...</p>
      </div>
    )
  }

  if (!canResetPassword) {
    return (
      <div className="space-y-6">
        <div className="space-y-2 text-center">
          <h2 className="text-2xl font-semibold">Password reset unavailable</h2>
          <p className="text-sm text-muted-foreground">
            Password reset is disabled for local accounts. If you require access, please contact
            your administrator for assistance.
          </p>
        </div>
        <div className="text-center text-sm text-muted-foreground">
          Return to{' '}
          <Link to="/login" className="text-primary hover:underline">
            login
          </Link>
          .
        </div>
      </div>
    )
  }

  if (completed) {
    return (
      <div className="space-y-6 text-center">
        <div className="space-y-2">
          <h2 className="text-2xl font-semibold">Password Updated</h2>
          <p className="text-sm text-muted-foreground">
            Your password has been reset successfully. You can now sign in with your new
            credentials.
          </p>
        </div>
        <div className="flex flex-col gap-3">
          <Button onClick={() => navigate('/login', { replace: true })} className="w-full">
            Go to Login
          </Button>
          <Link to="/login" className="text-sm text-primary hover:underline">
            Return to login
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2 text-center">
        <h2 className="text-2xl font-semibold">Choose a new password</h2>
        <p className="text-sm text-muted-foreground">
          Enter the reset token and your new password below.
        </p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Input
          {...register('token')}
          label="Reset Token"
          placeholder="Paste reset token"
          error={errors.token?.message}
        />

        <Input
          {...register('password')}
          label="New Password"
          type="password"
          placeholder="Enter a strong password"
          autoComplete="new-password"
          error={errors.password?.message}
        />

        <Input
          {...register('confirmPassword')}
          label="Confirm Password"
          type="password"
          placeholder="Re-enter password"
          autoComplete="new-password"
          error={errors.confirmPassword?.message}
        />

        {error && <p className="text-sm text-destructive">{error}</p>}

        <Button type="submit" className="w-full">
          Reset Password
        </Button>
      </form>
    </div>
  )
}
