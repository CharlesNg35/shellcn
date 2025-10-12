import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from 'react-router-dom'
import { setupSchema } from '@/schemas/auth'
import type { z } from 'zod'
import { useAuth } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { PasswordStrengthMeter } from '@/components/auth/PasswordStrengthMeter'
import { toast } from '@/lib/utils/toast'
import { registerLocal } from '@/lib/api/auth'

type RegisterFormData = z.infer<typeof setupSchema>

export function Register() {
  const navigate = useNavigate()
  const { providers, loadProviders, setupStatus, fetchSetupStatus, clearError } = useAuth({
    autoInitialize: false,
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    watch,
    setFocus,
  } = useForm<RegisterFormData>({
    resolver: zodResolver(setupSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
      firstName: '',
      lastName: '',
    },
  })

  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const passwordValue = watch('password')

  const canSelfRegister = useMemo(() => {
    const local = providers.find((provider) => provider.type === 'local')
    if (!local) {
      return false
    }
    return Boolean(local.allow_registration)
  }, [providers])

  useEffect(() => {
    if (!providers.length) {
      void loadProviders().catch(() => {
        /* ignore */
      })
    }
  }, [providers.length, loadProviders])

  useEffect(() => {
    setFocus('username')
  }, [setFocus])

  useEffect(() => {
    if (!setupStatus) {
      void fetchSetupStatus().catch(() => {
        /* ignore */
      })
    } else if (setupStatus.status === 'pending') {
      navigate('/setup', { replace: true })
      return
    }

    if (providers.length > 0 && !canSelfRegister) {
      navigate('/login', { replace: true })
    }

    return () => {
      clearError()
    }
  }, [canSelfRegister, providers.length, navigate, fetchSetupStatus, setupStatus, clearError])

  const onSubmit = async (data: RegisterFormData) => {
    setFormError(null)
    setSubmitting(true)

    try {
      const result = await registerLocal({
        username: data.username,
        email: data.email,
        password: data.password,
        first_name: data.firstName || undefined,
        last_name: data.lastName || undefined,
      })

      const query = result.requires_verification ? 'register_verify' : 'register_success'
      toast.success('Account created', {
        description:
          result.message ??
          (result.requires_verification
            ? 'Check your email to verify your account before signing in.'
            : 'You can now sign in with your new credentials.'),
      })
      navigate(`/login?notice=${query}`, { replace: true })
    } catch (error) {
      if (error instanceof Error) {
        setFormError(error.message)
      } else {
        setFormError('Unable to create account. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (!canSelfRegister) {
    return null
  }

  if (providers.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">Preparing registration…</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 className="text-2xl font-bold text-foreground">Create your account</h2>
        <p className="text-sm text-muted-foreground">
          Register with your work email to access ShellCN.
        </p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            {...register('firstName')}
            label="First Name"
            placeholder="Ada"
            error={errors.firstName?.message}
          />
          <Input
            {...register('lastName')}
            label="Last Name"
            placeholder="Lovelace"
            error={errors.lastName?.message}
          />
        </div>

        <Input
          {...register('username')}
          label="Username"
          placeholder="yourname"
          autoComplete="username"
          error={errors.username?.message}
        />

        <Input
          {...register('email')}
          label="Email"
          type="email"
          placeholder="you@example.com"
          autoComplete="email"
          error={errors.email?.message}
        />

        <Input
          {...register('password')}
          label="Password"
          type="password"
          autoComplete="new-password"
          placeholder="Enter a strong password"
          error={errors.password?.message}
        />

        {passwordValue ? <PasswordStrengthMeter password={passwordValue} /> : null}

        <Input
          {...register('confirmPassword')}
          label="Confirm Password"
          type="password"
          autoComplete="new-password"
          placeholder="Re-enter password"
          error={errors.confirmPassword?.message}
        />

        {formError ? (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
            {formError}
          </div>
        ) : null}

        <Button type="submit" className="w-full" loading={submitting}>
          Create account
        </Button>
      </form>

      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{' '}
        <Link to="/login" className="text-primary hover:underline">
          Sign in
        </Link>
      </p>
    </div>
  )
}
