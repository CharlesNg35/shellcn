import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { z } from 'zod'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { inviteAuthSettingsSchema } from '@/schemas/authProviders'
import { authProvidersApi } from '@/lib/api/auth-providers'
import { AUTH_PROVIDERS_QUERY_KEY, getAuthProviderDetailQueryKey } from '@/hooks/useAuthProviders'
import type { AuthProviderRecord } from '@/types/auth-providers'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'

type InviteSettingsFormValues = z.infer<typeof inviteAuthSettingsSchema>

interface InviteSettingsFormProps {
  provider?: AuthProviderRecord
  onCancel?: () => void
  onSuccess?: () => void
}

export function InviteSettingsForm({ provider, onCancel, onSuccess }: InviteSettingsFormProps) {
  const queryClient = useQueryClient()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const defaultValues = useMemo<InviteSettingsFormValues>(
    () => ({
      enabled: provider?.enabled ?? false,
      requireEmailVerification: provider?.requireEmailVerification ?? true,
    }),
    [provider]
  )

  const {
    control,
    handleSubmit,
    reset,
    formState: { isSubmitting },
  } = useForm<InviteSettingsFormValues>({
    resolver: zodResolver(inviteAuthSettingsSchema),
    defaultValues,
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const mutation = useMutation({
    mutationFn: authProvidersApi.updateInviteSettings,
    onSuccess: async () => {
      setFormError(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey('invite'),
        }),
      ])
      toast.success('Invitation settings updated')
      onSuccess?.()
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      setFormError(apiError)
      toast.error('Failed to update invite settings', {
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
          name="enabled"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Enable invitations</p>
                <p className="text-sm text-muted-foreground">
                  Allow administrators to invite new users via email.
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
                  Invited users must confirm their email before gaining access.
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
