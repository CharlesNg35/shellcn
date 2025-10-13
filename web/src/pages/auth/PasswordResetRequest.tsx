import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { Link } from 'react-router-dom'
import { passwordResetRequestSchema } from '@/schemas/auth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/hooks/useAuth'

type PasswordResetRequestFormData = z.infer<typeof passwordResetRequestSchema>

export function PasswordResetRequest() {
  const { requestPasswordReset, providers, loadProviders } = useAuth({ autoInitialize: false })
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState<string | null>(null)
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
  } = useForm<PasswordResetRequestFormData>({
    resolver: zodResolver(passwordResetRequestSchema),
    defaultValues: {
      email: '',
    },
  })

  const onSubmit = async (data: PasswordResetRequestFormData) => {
    setError(null)

    try {
      await requestPasswordReset({ email: data.email, identifier: data.email })
      setSubmitted(true)
    } catch (requestError) {
      if (requestError instanceof Error) {
        setError(requestError.message)
      } else {
        setError('Unable to process reset request. Please try again later.')
      }
    }
  }

  if (!providersReady) {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">Preparing reset flow...</p>
      </div>
    )
  }

  if (!canResetPassword) {
    return (
      <div className="space-y-6">
        <div className="space-y-2 text-center">
          <h2 className="text-2xl font-semibold">Password reset unavailable</h2>
          <p className="text-sm text-muted-foreground">
            Password reset is disabled for local accounts. Please contact your administrator if you
            need assistance accessing ShellCN.
          </p>
        </div>

        <div className="text-center text-sm text-muted-foreground">
          Remembered your password?{' '}
          <Link to="/login" className="text-primary hover:underline">
            Back to login
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2 text-center">
        <h2 className="text-2xl font-semibold">Reset your password</h2>
        <p className="text-sm text-muted-foreground">
          Enter your email address and we&apos;ll send instructions to reset your password.
        </p>
      </div>

      {submitted ? (
        <div className="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-emerald-700">
          If an account exists for the provided email, you will receive password reset instructions
          shortly. Please check your inbox.
        </div>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <Input
            {...register('email')}
            label="Email"
            type="email"
            placeholder="you@example.com"
            autoComplete="email"
            error={errors.email?.message}
          />

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button type="submit" className="w-full">
            Send Reset Link
          </Button>
        </form>
      )}

      <div className="text-center text-sm text-muted-foreground">
        Remembered your password?{' '}
        <Link to="/login" className="text-primary hover:underline">
          Back to login
        </Link>
      </div>
    </div>
  )
}
