import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { inviteAcceptSchema } from '@/schemas/auth'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { invitesApi } from '@/lib/api/invites'
import type { InviteInfoResponse } from '@/types/invites'
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
  const [inviteInfo, setInviteInfo] = useState<InviteInfoResponse | null>(null)
  const [isLoadingInfo, setIsLoadingInfo] = useState(false)
  const [infoError, setInfoError] = useState<string | null>(null)

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
  const effectiveExistingAccount = inviteInfo?.has_account ? true : existingAccount

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
    if (effectiveExistingAccount) {
      setValue('username', '')
      setValue('password', '')
      setValue('confirmPassword', '')
      clearErrors(['username', 'password', 'confirmPassword'])
    }
  }, [effectiveExistingAccount, setValue, clearErrors])

  useEffect(() => {
    if (providers.length === 0) {
      void loadProviders().catch(() => {
        /* handled globally */
      })
    }
  }, [providers.length, loadProviders])

  useEffect(() => {
    const token = params.get('token')
    if (!token) {
      return
    }
    setIsLoadingInfo(true)
    setInfoError(null)
    invitesApi
      .info(token)
      .then((info) => {
        setInviteInfo(info)
        if (info.has_account) {
          setValue('existingAccount', true, { shouldDirty: false, shouldValidate: false })
        }
      })
      .catch((error) => {
        const apiError = toApiError(error)
        setInfoError(apiError.message)
      })
      .finally(() => {
        setIsLoadingInfo(false)
      })
  }, [params, setValue])

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
    if (isAuthenticated && !effectiveExistingAccount) {
      setValue('existingAccount', true, { shouldDirty: false, shouldValidate: false })
    }
  }, [isAuthenticated, effectiveExistingAccount, setValue])

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
  const inviteTeamName = inviteInfo?.team_name
  const inviteProvider = inviteInfo?.provider

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
        username: effectiveExistingAccount ? undefined : values.username,
        password: effectiveExistingAccount ? undefined : values.password,
      }

      const result = await invitesApi.redeem(payload)

      const providerType = (result.user.provider || 'local').toLowerCase()
      const providerMeta = providers.find((provider) => provider.type === providerType)

      toast.success('Invitation complete', {
        description:
          result.message ??
          (inviteTeamName
            ? `You now have access to ${inviteTeamName}.`
            : 'You can now access ShellCN.'),
      })

      if (
        result.created_user &&
        !effectiveExistingAccount &&
        providerType === 'local' &&
        canAutoLogin
      ) {
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
        const ssoLogin =
          providerMeta.login_url ||
          `/api/auth/providers/${providerType}/login?redirect=${encodeURIComponent(currentInvitePath)}&error_redirect=${encodeURIComponent(ssoErrorRedirect)}`
        window.location.href = ssoLogin
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
          {inviteInfo?.has_account
            ? 'This email already has a ShellCN account. Join the team using your existing credentials.'
            : 'Create a new ShellCN account or link this invitation to an existing user profile.'}
        </p>
        {inviteTeamName ? (
          <p className="text-sm text-muted-foreground">
            Team: <span className="font-medium">{inviteTeamName}</span>
          </p>
        ) : null}
        {infoError ? <p className="text-sm text-destructive">{infoError}</p> : null}
      </div>

      <form onSubmit={onSubmit} className="space-y-4">
        <Input
          label="Invitation Token"
          {...register('token')}
          error={errors.token?.message}
          readOnly={Boolean(params.get('token'))}
        />

        {allowLocalRegistration && !isAuthenticated && !inviteInfo?.has_account ? (
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

        {effectiveExistingAccount ? (
          <div className="space-y-3">
            {isAuthenticated ? (
              <div className="rounded-md border border-green-300/60 bg-green-50 p-3 text-sm text-green-900">
                Signed in as <span className="font-semibold">{authenticatedEmail}</span>. Submit to
                join the team linked to this invitation.
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  {inviteProvider
                    ? `Sign in with the ${inviteProvider} provider linked to your ShellCN account, then return to this page to finish joining the team.`
                    : 'Sign in with the provider linked to your ShellCN account, then return to this page to finish the process.'}
                </p>
                {providers.some((provider) => (provider.flow ?? 'password') === 'redirect') ? (
                  <SSOButtons
                    providers={providers}
                    successRedirect={currentInvitePath}
                    errorRedirect={ssoErrorRedirect}
                    disabled={isSubmitting || isLoadingInfo}
                  />
                ) : null}
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

        <Button
          type="submit"
          className="w-full"
          loading={isSubmitting}
          disabled={isSubmitting || isLoadingInfo}
        >
          {effectiveExistingAccount ? 'Join Team' : 'Activate Account'}
        </Button>
      </form>
    </div>
  )
}
