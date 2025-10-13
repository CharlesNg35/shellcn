import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { z } from 'zod'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { localAuthSettingsSchema } from '@/schemas/authProviders'
import { authProvidersApi } from '@/lib/api/auth-providers'
import { AUTH_PROVIDERS_QUERY_KEY, getAuthProviderDetailQueryKey } from '@/hooks/useAuthProviders'
import type { AuthProviderRecord } from '@/types/auth-providers'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'

type LocalSettingsFormValues = z.infer<typeof localAuthSettingsSchema>

interface LocalSettingsFormProps {
  provider?: AuthProviderRecord
  onCancel?: () => void
  onSuccess?: () => void
}

export function LocalSettingsForm({ provider, onCancel, onSuccess }: LocalSettingsFormProps) {
  const queryClient = useQueryClient()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const defaultValues = useMemo<LocalSettingsFormValues>(
    () => ({
      allowRegistration: provider?.allowRegistration ?? false,
      requireEmailVerification: provider?.requireEmailVerification ?? true,
      allowPasswordReset: provider?.allowPasswordReset ?? true,
    }),
    [provider]
  )

  const {
    control,
    handleSubmit,
    formState: { isSubmitting },
    reset,
  } = useForm<LocalSettingsFormValues>({
    resolver: zodResolver(localAuthSettingsSchema),
    defaultValues,
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const mutation = useMutation({
    mutationFn: authProvidersApi.updateLocalSettings,
    onSuccess: async () => {
      setFormError(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey('local'),
        }),
      ])
      toast.success('Local authentication settings updated')
      onSuccess?.()
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      setFormError(apiError)
      toast.error('Failed to update local authentication', {
        description: apiError.message,
      })
    },
  })

  const onSubmit = handleSubmit(async (values) => {
    await mutation.mutateAsync(values)
  })

  return (
    <form className="space-y-6" onSubmit={onSubmit}>
      <div className="space-y-4">
        <Controller
          control={control}
          name="allowRegistration"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Allow self-registration</p>
                <p className="text-sm text-muted-foreground">
                  When enabled, new users can create local accounts without an invitation.
                </p>
              </div>
            </label>
          )}
        />

        <Controller
          control={control}
          name="requireEmailVerification"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Require email verification</p>
                <p className="text-sm text-muted-foreground">
                  Users must confirm their email address before activating a local account.
                </p>
              </div>
            </label>
          )}
        />

        <Controller
          control={control}
          name="allowPasswordReset"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Enable password reset emails</p>
                <p className="text-sm text-muted-foreground">
                  Allow users to request password reset links via email.
                </p>
              </div>
            </label>
          )}
        />
      </div>

      {formError ? (
        <div className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {formError.message}
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={isSubmitting || mutation.isPending}
        >
          Cancel
        </Button>
        <Button type="submit" loading={isSubmitting || mutation.isPending}>
          Save Changes
        </Button>
      </div>
    </form>
  )
}
