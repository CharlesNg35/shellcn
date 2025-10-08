import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { loginSchema } from '@/schemas/auth'
import { useAuth } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'

type LoginFormData = z.infer<typeof loginSchema>

export function Login() {
  const navigate = useNavigate()
  const { login, isLoading, error, clearError, isMfaRequired, fetchSetupStatus, status } = useAuth()
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

  const [checkingSetup, setCheckingSetup] = useState(true)

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
        if (setup.status === 'pending') {
          navigate('/setup', { replace: true })
        } else {
          setCheckingSetup(false)
        }
      })
      .catch(() => {
        if (subscribed) {
          setCheckingSetup(false)
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

  if (checkingSetup) {
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

        <div className="flex items-center justify-between text-sm">
          <Link to="/password-reset" className="text-primary hover:underline">
            Forgot password?
          </Link>
        </div>

        {error && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <Button type="submit" className="w-full" loading={isLoading}>
          Sign In
        </Button>
      </form>

      <div className="text-center text-sm text-muted-foreground">
        First time here?{' '}
        <Link to="/setup" className="text-primary hover:underline">
          Complete initial setup
        </Link>
      </div>
    </div>
  )
}
