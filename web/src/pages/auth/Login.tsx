import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import type { SetupStatusPayload } from '@/types/auth'
import { loginSchema } from '@/schemas/auth'
import { useAuth } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import {} from '@/lib/constants'

type LoginFormData = z.infer<typeof loginSchema>

export function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const {
    login,
    isLoading,
    error,
    clearError,
    isMfaRequired,
    fetchSetupStatus,
    status,
    providers,
    loadProviders,
    setupStatus,
    mfaChallenge,
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

  const [setupState, setSetupState] = useState<'checking' | 'pending' | 'complete'>(
    setupStatus?.status ?? 'checking'
  )
  const [selectedProvider, setSelectedProvider] = useState<string>('local')
  const [ssoError, setSsoError] = useState<string | null>(null)
  const [mfaErrorMessage, setMfaErrorMessage] = useState<string | null>(null)

  const passwordProviders = useMemo(
    () =>
      providers.filter(
        (provider) => provider.enabled && (provider.flow ?? 'password') === 'password'
      ),
    [providers]
  )

  useEffect(() => {
    if (providers.length === 0) {
      void loadProviders()
    }
  }, [providers.length, loadProviders])

  useEffect(() => {
    if (passwordProviders.length === 0) {
      setSelectedProvider('local')
      return
    }

    setSelectedProvider((current) => {
      if (passwordProviders.some((provider) => provider.type === current)) {
        return current
      }
      const localProvider = passwordProviders.find((provider) => provider.type === 'local')
      return localProvider?.type ?? passwordProviders[0].type
    })
  }, [passwordProviders])

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
    const params = new URLSearchParams(location.search)
    const reason = params.get('error_reason')
    const notice = params.get('notice')
    switch (reason) {
      case 'provider_mismatch':
        setSsoError(
          'This account is already linked to a different sign-in provider. Please use the original provider or contact an administrator.'
        )
        break
      case 'user_disabled':
        setSsoError(
          'Your account has been disabled. Please contact an administrator for assistance.'
        )
        break
      case 'email_required':
        setSsoError(
          'The identity provider did not return an email address. Please contact your administrator.'
        )
        break
      case 'not_found':
        setSsoError(
          'We could not match your identity to an existing account. If you expect access, contact your administrator.'
        )
        break
      default:
        setSsoError(null)
    }

    if (notice === 'mfa_failed') {
      setMfaErrorMessage('Verification code is invalid. Please try again.')
    } else {
      setMfaErrorMessage(null)
    }
  }, [location.search])

  useEffect(() => {
    let active = true

    const applyStatus = (result: SetupStatusPayload | null | undefined) => {
      if (!active) {
        return
      }
      if (!result) {
        setSetupState('complete')
        return
      }
      setSetupState(result.status)
      if (result.status === 'pending' && location.pathname !== '/setup') {
        navigate('/setup', { replace: true })
      }
    }

    if (setupStatus) {
      applyStatus(setupStatus)
      return () => {
        active = false
        clearError()
      }
    }

    setSetupState('checking')
    fetchSetupStatus()
      .then(applyStatus)
      .catch(() => {
        if (active) {
          setSetupState('complete')
        }
      })

    return () => {
      active = false
      clearError()
      setMfaErrorMessage(null)
    }
  }, [setupStatus, fetchSetupStatus, navigate, clearError, location.pathname])

  useEffect(() => {
    if (status === 'authenticated') {
      navigate('/dashboard', { replace: true })
    } else if (isMfaRequired && mfaChallenge) {
      navigate('/mfa', { replace: true })
    }
  }, [status, isMfaRequired, mfaChallenge, navigate])

  const onSubmit = async (data: LoginFormData) => {
    clearError()
    setMfaErrorMessage(null)

    try {
      const result = await login({
        identifier: data.identifier,
        password: data.password,
        remember_device: data.remember_device,
        provider: selectedProvider,
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
        {passwordProviders.length > 1 ? (
          <div className="flex flex-wrap gap-2 pt-2">
            {passwordProviders.map((provider) => (
              <Button
                key={provider.type}
                type="button"
                variant={selectedProvider === provider.type ? 'secondary' : 'outline'}
                size="sm"
                onClick={() => setSelectedProvider(provider.type)}
              >
                {provider.name}
              </Button>
            ))}
          </div>
        ) : null}
        {passwordProviders.length === 1 ? (
          <p className="text-xs text-muted-foreground">
            Signing in with {passwordProviders[0].name}
          </p>
        ) : null}
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

        {(error || mfaErrorMessage) && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
            <p className="font-medium">{error ?? mfaErrorMessage}</p>
          </div>
        )}

        {ssoError && (
          <div className="rounded-lg border border-amber-300/60 bg-amber-50 p-3 text-sm text-amber-900">
            <p className="font-medium">{ssoError}</p>
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
