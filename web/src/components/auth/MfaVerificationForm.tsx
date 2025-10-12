import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { mfaVerificationSchema } from '@/schemas/auth'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { useAuth } from '@/hooks/useAuth'

type MfaFormData = z.infer<typeof mfaVerificationSchema>

interface MfaVerificationFormProps {
  onSuccess?: () => void
  onFailure?: (error: Error) => void
}

export function MfaVerificationForm({ onSuccess, onFailure }: MfaVerificationFormProps) {
  const { mfaChallenge, verifyMfa, error, clearError, isLoading } = useAuth()

  const {
    register,
    handleSubmit,
    formState: { errors },
    setFocus,
  } = useForm<MfaFormData>({
    resolver: zodResolver(mfaVerificationSchema),
    defaultValues: { code: '' },
  })

  useEffect(() => {
    setFocus('code')
  }, [setFocus])

  useEffect(() => {
    return () => {
      clearError()
    }
  }, [clearError])

  if (!mfaChallenge) {
    return null
  }

  const onSubmit = async (data: MfaFormData) => {
    try {
      await verifyMfa({
        challenge_id: mfaChallenge.challenge_id,
        mfa_token: data.code,
      })

      onSuccess?.()
    } catch (err) {
      if (onFailure && err instanceof Error) {
        onFailure(err)
        return
      }
      throw err
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div className="space-y-2 text-center">
        <h2 className="text-2xl font-semibold">Multi-factor Authentication</h2>
        <p className="text-sm text-muted-foreground">
          Enter the verification code from your {mfaChallenge.method?.toUpperCase()} device.
        </p>
      </div>

      <Input
        {...register('code')}
        label="Verification Code"
        inputMode="numeric"
        autoComplete="one-time-code"
        placeholder="Enter your 6-digit code"
        error={errors.code?.message}
      />

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Button type="submit" className="w-full" loading={isLoading}>
        Verify
      </Button>
    </form>
  )
}
