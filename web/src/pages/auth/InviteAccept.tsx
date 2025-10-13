import { useEffect, useMemo } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
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

type InviteAcceptFormValues = z.infer<typeof inviteAcceptSchema>

export function InviteAccept() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { login, clearError, providers, loadProviders } = useAuth({ autoInitialize: false })

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

  const canAutoLogin = useMemo(() => {
    return providers.some(
      (provider) =>
        provider.enabled &&
        (provider.flow ?? 'password') === 'password' &&
        provider.type === 'local'
    )
  }, [providers])

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

      toast.success('Invitation complete', {
        description: result.message ?? 'You can now access ShellCN.',
      })

      if (result.created_user && !existingAccount && canAutoLogin) {
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
          navigate('/login', { replace: true })
        }
      } else {
        navigate('/login', { replace: true })
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

        {!existingAccount ? (
          <Input
            label="Username"
            placeholder="your.username"
            autoComplete="username"
            {...register('username')}
            error={errors.username?.message}
          />
        ) : null}

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

        {!existingAccount ? (
          <>
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
        ) : null}

        <Button type="submit" className="w-full" loading={isSubmitting}>
          {existingAccount ? 'Join Team' : 'Activate Account'}
        </Button>
      </form>
    </div>
  )
}
