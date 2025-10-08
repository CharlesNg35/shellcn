import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { setupSchema } from '@/schemas/auth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { PasswordStrengthMeter } from '@/components/auth/PasswordStrengthMeter'
import { useAuth } from '@/hooks/useAuth'
import {} from '@/lib/constants'

type SetupFormData = z.infer<typeof setupSchema>

export function Setup() {
  const navigate = useNavigate()
  const { completeSetup, fetchSetupStatus } = useAuth({ autoInitialize: false })
  const {
    register,
    handleSubmit,
    formState: { errors },
    watch,
    setFocus,
  } = useForm<SetupFormData>({
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
  const [statusChecked, setStatusChecked] = useState(false)

  const passwordValue = watch('password')

  useEffect(() => {
    setFocus('username')
  }, [setFocus])

  useEffect(() => {
    let active = true
    fetchSetupStatus()
      .then((status) => {
        if (!active) {
          return
        }

        if (status.status === 'complete') {
          navigate('/login', { replace: true })
        } else {
          setStatusChecked(true)
        }
      })
      .catch(() => {
        if (active) {
          setStatusChecked(true)
        }
      })

    return () => {
      active = false
    }
  }, [fetchSetupStatus, navigate])

  const onSubmit = async (data: SetupFormData) => {
    setFormError(null)
    setSubmitting(true)

    try {
      await completeSetup({
        username: data.username,
        email: data.email,
        password: data.password,
        first_name: data.firstName || undefined,
        last_name: data.lastName || undefined,
      })

      navigate('/login', {
        replace: true,
      })
    } catch (error) {
      if (error instanceof Error) {
        setFormError(error.message)
      } else {
        setFormError('Failed to complete setup. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (!statusChecked) {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">Preparing setup wizard...</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 className="text-2xl font-bold text-foreground">Initial Setup</h2>
        <p className="text-sm text-muted-foreground">
          Create the first administrator account to get started
        </p>
      </div>

      <div className="rounded-lg border border-primary/20 bg-primary/5 p-4">
        <p className="text-sm text-foreground">
          <strong>Note:</strong> This account will have full administrative privileges. Choose a
          strong password and keep the credentials secure.
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
          placeholder="admin"
          autoComplete="username"
          error={errors.username?.message}
        />

        <Input
          {...register('email')}
          label="Email"
          type="email"
          placeholder="admin@example.com"
          autoComplete="email"
          error={errors.email?.message}
        />

        <div>
          <Input
            {...register('password')}
            label="Password"
            type="password"
            autoComplete="new-password"
            placeholder="Enter a strong password"
            error={errors.password?.message}
          />
          {passwordValue && <PasswordStrengthMeter password={passwordValue} />}
        </div>

        <Input
          {...register('confirmPassword')}
          label="Confirm Password"
          type="password"
          autoComplete="new-password"
          placeholder="Re-enter password"
          error={errors.confirmPassword?.message}
        />

        {formError && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
            {formError}
          </div>
        )}

        <Button type="submit" className="w-full" loading={submitting}>
          Create Administrator Account
        </Button>
      </form>
    </div>
  )
}
