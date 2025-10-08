import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { loginSchema } from '@/schemas/auth'
import { useAuth } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import {} from '@/lib/constants'

type LoginFormData = z.infer<typeof loginSchema>

export function Login() {
  const navigate = useNavigate()
  const {
    login,
    isLoading,
    error,
    clearError,
    isMfaRequired,
    fetchSetupStatus,
    status,
    providers,
  } = useAuth()
  const {
    register,
    handleSubmit,
    formState: { errors },
    setFocus,
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      identifier: '',
      password: '',
      remember_device: false,
    },
  })

  const [setupState, setSetupState] = useState<'checking' | 'pending' | 'complete'>('checking')

  const canResetPassword = useMemo(() => {
    const localProvider = providers.find((provider) => provider.type === 'local')
    if (!localProvider) {
      return false
    }
    const flag = localProvider.allow_password_reset
    return flag === undefined ? true : Boolean(flag)
  }, [providers])

  useEffect(() => {
    setFocus('identifier')
  }, [setFocus])

  useEffect(() => {
    let subscribed = true
    fetchSetupStatus()
      .then((setup) => {
        if (!subscribed) {
          return
        }
        if (!setup || setup.status === 'pending') {
          setSetupState('pending')
          navigate('/setup', { replace: true })
        } else {
          setSetupState('complete')
        }
      })
      .catch(() => {
        if (subscribed) {
          setSetupState('complete')
        }
      })

    return () => {
      subscribed = false
      clearError()
    }
  }, [fetchSetupStatus, navigate, clearError])

  useEffect(() => {
    if (status === 'authenticated') {
      navigate('/dashboard', { replace: true })
    } else if (isMfaRequired) {
      navigate('/mfa', { replace: true })
    }
  }, [status, isMfaRequired, navigate])

  const onSubmit = async (data: LoginFormData) => {
    clearError()

    try {
      const result = await login({
        identifier: data.identifier,
        password: data.password,
        remember_device: data.remember_device,
      })

      if (!result.mfaRequired) {
        navigate('/dashboard', { replace: true })
      } else {
        navigate('/mfa', { replace: true })
      }
    } catch (submitError) {
      console.error('Login failed', submitError)
    }
  }

  if (setupState === 'checking') {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">Checking system status...</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 className="text-2xl font-bold text-foreground">Sign in</h2>
        <p className="text-sm text-muted-foreground">Enter your credentials to access ShellCN</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Input
          {...register('identifier')}
          label="Username or Email"
          placeholder="admin or admin@example.com"
          autoComplete="username"
          error={errors.identifier?.message}
        />

        <Input
          {...register('password')}
          label="Password"
          type="password"
          placeholder="Enter your password"
          autoComplete="current-password"
          error={errors.password?.message}
        />

        {canResetPassword ? (
          <div className="flex items-center justify-between text-sm">
            <Link to="/password-reset" className="text-primary hover:underline">
              Forgot password?
            </Link>
          </div>
        ) : null}

        {error && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <Button type="submit" className="w-full" loading={isLoading}>
          Sign In
        </Button>
      </form>

      {setupState === 'pending' && (
        <div className="text-center text-sm text-muted-foreground">
          First time here?{' '}
          <Link to="/setup" className="text-primary hover:underline">
            Complete initial setup
          </Link>
        </div>
      )}
    </div>
  )
}
