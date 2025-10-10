import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { inviteAcceptSchema } from '@/schemas/auth'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { invitesApi } from '@/lib/api/invites'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import { useAuth } from '@/hooks/useAuth'

type InviteAcceptFormValues = z.infer<typeof inviteAcceptSchema>

export function InviteAccept() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { login, clearError } = useAuth({ autoInitialize: false })

  const {
    register,
    handleSubmit,
    setValue,
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
    },
  })

  useEffect(() => {
    const token = params.get('token')
    if (token) {
      setValue('token', token)
    }

    return () => {
      clearError()
    }
  }, [params, setValue, clearError])

  const onSubmit = handleSubmit(async (values) => {
    try {
      const firstName = values.firstName?.trim()
      const lastName = values.lastName?.trim()

      await invitesApi.redeem({
        token: values.token,
        username: values.username,
        password: values.password,
        first_name: firstName ? firstName : undefined,
        last_name: lastName ? lastName : undefined,
      })

      toast.success('Account ready', {
        description: 'Welcome! Redirecting to your dashboard...',
      })

      await login({
        identifier: values.username,
        password: values.password,
      })

      navigate('/dashboard', { replace: true })
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
          Choose your username and password to finish setting up your ShellCN account.
        </p>
      </div>

      <form onSubmit={onSubmit} className="space-y-4">
        <Input
          label="Invitation Token"
          {...register('token')}
          error={errors.token?.message}
          readOnly={Boolean(params.get('token'))}
        />

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

        <Button type="submit" className="w-full" loading={isSubmitting}>
          Activate Account
        </Button>
      </form>
    </div>
  )
}
