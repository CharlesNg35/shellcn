import { useEffect, useMemo } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { inviteAcceptSchema } from '@/schemas/auth'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { invitesApi } from '@/lib/api/invites'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import { useAuth } from '@/hooks/useAuth'
import { Checkbox } from '@/components/ui/Checkbox'
import { SSOButtons } from '@/components/auth/SSOButtons'

type InviteAcceptFormValues = z.infer<typeof inviteAcceptSchema>

export function InviteAccept() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { login, clearError, providers, loadProviders, user, isAuthenticated } = useAuth({
    autoInitialize: false,
  })

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    control,
    clearErrors,
    formState: { errors, isSubmitting },
  } = useForm<InviteAcceptFormValues>({
    resolver: zodResolver(inviteAcceptSchema),
    defaultValues: {
      token: params.get('token') ?? '',
      username: '',
      password: '',
      confirmPassword: '',
      firstName: '',
      lastName: '',
      existingAccount: false,
    },
  })

  const existingAccount = watch('existingAccount')

  useEffect(() => {
    const token = params.get('token')
    if (token) {
      setValue('token', token)
    }

    return () => {
      clearError()
    }
  }, [params, setValue, clearError])

  useEffect(() => {
    if (existingAccount) {
      setValue('username', '')
      setValue('password', '')
      setValue('confirmPassword', '')
      clearErrors(['username', 'password', 'confirmPassword'])
    }
  }, [existingAccount, setValue, clearErrors])

  useEffect(() => {
    if (providers.length === 0) {
      void loadProviders().catch(() => {
        /* handled globally */
      })
    }
  }, [providers.length, loadProviders])

  const localPasswordProvider = useMemo(
    () =>
      providers.find(
        (provider) =>
          provider.type === 'local' &&
          provider.enabled &&
          (provider.flow ?? 'password') === 'password'
      ),
    [providers]
  )

  const allowLocalRegistration = Boolean(
    localPasswordProvider && (localPasswordProvider.allow_registration ?? true)
  )

  useEffect(() => {
    if ((isAuthenticated || !allowLocalRegistration) && !existingAccount) {
      setValue('existingAccount', true)
    }
  }, [isAuthenticated, allowLocalRegistration, existingAccount, setValue])

  useEffect(() => {
    if (params.get('notice') === 'sso_failed') {
      toast.error('Single sign-on failed', {
        description: 'Please try again or contact your administrator.',
      })
    }
  }, [params])

  const canAutoLogin = useMemo(() => {
    return providers.some(
      (provider) =>
        provider.enabled &&
        (provider.flow ?? 'password') === 'password' &&
        provider.type === 'local'
    )
  }, [providers])

  const authenticatedEmail = user?.email ?? ''

  const currentInvitePath = useMemo(() => {
    if (typeof window === 'undefined') {
      return '/invite/accept'
    }
    return window.location.pathname + window.location.search
  }, [])

  const inviteToken = params.get('token') ?? ''
  const ssoErrorRedirect = `/invite/accept?token=${encodeURIComponent(inviteToken)}&notice=sso_failed`

  const onSubmit = handleSubmit(async (values) => {
    try {
      const firstName = values.firstName?.trim()
      const lastName = values.lastName?.trim()

      const payload = {
        token: values.token,
        first_name: firstName ? firstName : undefined,
        last_name: lastName ? lastName : undefined,
        username: existingAccount ? undefined : values.username,
        password: existingAccount ? undefined : values.password,
      }

      const result = await invitesApi.redeem(payload)

      const providerType = (result.user.provider || 'local').toLowerCase()
      const providerMeta = providers.find((provider) => provider.type === providerType)

      toast.success('Invitation complete', {
        description: result.message ?? 'You can now access ShellCN.',
      })

      if (result.created_user && !existingAccount && providerType === 'local' && canAutoLogin) {
        try {
          await login({
            identifier: values.username ?? '',
            password: values.password ?? '',
          })
          navigate('/dashboard', { replace: true })
        } catch (authError) {
          const authIssue = toApiError(authError)
          toast.info('Account created', {
            description: authIssue.message || 'Sign in with your new credentials to continue.',
          })
          navigate(`/login?notice=team-invite&provider=${providerType}`, { replace: true })
        }
      } else if (!result.created_user && providerMeta && providerMeta.flow === 'redirect') {
        if (providerMeta.login_url) {
          window.location.href = providerMeta.login_url
        } else {
          window.location.href = `/api/auth/providers/${providerType}/login?redirect=${encodeURIComponent(currentInvitePath)}&error_redirect=${encodeURIComponent(ssoErrorRedirect)}`
        }
      } else {
        navigate(`/login?notice=team-invite&provider=${providerType}`, { replace: true })
      }
    } catch (error) {
      const apiError = toApiError(error)
      toast.error('Unable to complete invitation', {
        description: apiError.message,
      })
    }
  })

  return (
    <div className="space-y-6">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-semibold text-foreground">Complete Your Invitation</h1>
        <p className="text-sm text-muted-foreground">
          Create a new ShellCN account or link this invitation to an existing user profile.
        </p>
      </div>

      <form onSubmit={onSubmit} className="space-y-4">
        <Input
          label="Invitation Token"
          {...register('token')}
          error={errors.token?.message}
          readOnly={Boolean(params.get('token'))}
        />

        {allowLocalRegistration && !isAuthenticated ? (
          <Controller
            name="existingAccount"
            control={control}
            render={({ field }) => (
              <div className="flex items-center space-x-3 rounded-md border border-border/60 bg-muted/30 p-3">
                <Checkbox
                  id="existing-account"
                  checked={field.value ?? false}
                  onCheckedChange={(checked) => field.onChange(Boolean(checked))}
                />
                <label htmlFor="existing-account" className="text-sm text-foreground">
                  I already have a ShellCN account
                </label>
              </div>
            )}
          />
        ) : null}

        {existingAccount ? (
          <div className="space-y-3">
            {isAuthenticated ? (
              <div className="rounded-md border border-green-300/60 bg-green-50 p-3 text-sm text-green-900">
                Signed in as <span className="font-semibold">{authenticatedEmail}</span>. Submit to
                join the team linked to this invitation.
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  Sign in with the provider linked to your ShellCN account, then return to this page
                  to finish joining the team.
                </p>
                <SSOButtons
                  providers={providers}
                  successRedirect={currentInvitePath}
                  errorRedirect={ssoErrorRedirect}
                  disabled={isSubmitting}
                />
                <div className="text-sm text-muted-foreground">
                  Prefer password sign-in?{' '}
                  <Link to="/login" className="text-primary hover:underline">
                    Go to the login page
                  </Link>
                  .
                </div>
              </div>
            )}
          </div>
        ) : (
          <>
            <Input
              label="Username"
              placeholder="your.username"
              autoComplete="username"
              {...register('username')}
              error={errors.username?.message}
            />

            <div className="grid gap-4 md:grid-cols-2">
              <Input
                label="First Name"
                autoComplete="given-name"
                {...register('firstName')}
                error={errors.firstName?.message}
              />
              <Input
                label="Last Name"
                autoComplete="family-name"
                {...register('lastName')}
                error={errors.lastName?.message}
              />
            </div>

            <Input
              label="Password"
              type="password"
              autoComplete="new-password"
              {...register('password')}
              error={errors.password?.message}
            />

            <Input
              label="Confirm Password"
              type="password"
              autoComplete="new-password"
              {...register('confirmPassword')}
              error={errors.confirmPassword?.message}
            />
          </>
        )}

        <Button type="submit" className="w-full" loading={isSubmitting}>
          {existingAccount ? 'Join Team' : 'Activate Account'}
        </Button>
      </form>
    </div>
  )
}
