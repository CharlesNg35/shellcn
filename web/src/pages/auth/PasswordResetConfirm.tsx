import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { passwordResetConfirmSchema } from '@/schemas/auth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/hooks/useAuth'

type PasswordResetConfirmFormData = z.infer<typeof passwordResetConfirmSchema>

export function PasswordResetConfirm() {
  const navigate = useNavigate()
  const { confirmPasswordReset } = useAuth({ autoInitialize: false })
  const [params] = useSearchParams()
  const prefilledToken = useMemo(() => params.get('token') ?? '', [params])
  const [error, setError] = useState<string | null>(null)
  const [completed, setCompleted] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<PasswordResetConfirmFormData>({
    resolver: zodResolver(passwordResetConfirmSchema),
    defaultValues: {
      token: prefilledToken,
      password: '',
      confirmPassword: '',
    },
  })

  const onSubmit = async (data: PasswordResetConfirmFormData) => {
    setError(null)

    try {
      await confirmPasswordReset({
        token: data.token,
        password: data.password,
      })
      setCompleted(true)
    } catch (submitError) {
      if (submitError instanceof Error) {
        setError(submitError.message)
      } else {
        setError('Unable to reset password. Please try again or request a new link.')
      }
    }
  }

  if (completed) {
    return (
      <div className="space-y-6 text-center">
        <div className="space-y-2">
          <h2 className="text-2xl font-semibold">Password Updated</h2>
          <p className="text-sm text-muted-foreground">
            Your password has been reset successfully. You can now sign in with your new
            credentials.
          </p>
        </div>
        <div className="flex flex-col gap-3">
          <Button onClick={() => navigate('/login', { replace: true })} className="w-full">
            Go to Login
          </Button>
          <Link to="/login" className="text-sm text-primary hover:underline">
            Return to login
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2 text-center">
        <h2 className="text-2xl font-semibold">Choose a new password</h2>
        <p className="text-sm text-muted-foreground">
          Enter the reset token and your new password below.
        </p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Input
          {...register('token')}
          label="Reset Token"
          placeholder="Paste reset token"
          error={errors.token?.message}
        />

        <Input
          {...register('password')}
          label="New Password"
          type="password"
          placeholder="Enter a strong password"
          autoComplete="new-password"
          error={errors.password?.message}
        />

        <Input
          {...register('confirmPassword')}
          label="Confirm Password"
          type="password"
          placeholder="Re-enter password"
          autoComplete="new-password"
          error={errors.confirmPassword?.message}
        />

        {error && <p className="text-sm text-destructive">{error}</p>}

        <Button type="submit" className="w-full">
          Reset Password
        </Button>
      </form>
    </div>
  )
}
