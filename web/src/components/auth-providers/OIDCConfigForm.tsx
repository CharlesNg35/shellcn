import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { oidcConfigSchema } from '@/schemas/authProviders'
import { authProvidersApi } from '@/lib/api/auth-providers'
import {
  AUTH_PROVIDERS_QUERY_KEY,
  getAuthProviderDetailQueryKey,
  useAuthProviderDetails,
} from '@/hooks/useAuthProviders'
import type { AuthProviderRecord } from '@/types/auth-providers'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'

type OIDCConfigFormValues = z.infer<typeof oidcConfigSchema>

interface OIDCConfigFormProps {
  provider?: AuthProviderRecord
  onCancel?: () => void
  onSuccess?: () => void
}

const DEFAULT_SCOPES = 'openid profile email'

function getDefaultRedirectUrl() {
  if (typeof window === 'undefined') {
    return ''
  }
  return `${window.location.origin}/api/auth/providers/oidc/callback`
}

export function OIDCConfigForm({ provider, onCancel, onSuccess }: OIDCConfigFormProps) {
  const queryClient = useQueryClient()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const detailsQuery = useAuthProviderDetails('oidc', {
    enabled: Boolean(provider),
  })

  const defaultValues = useMemo<OIDCConfigFormValues>(() => {
    const detail = detailsQuery.data
    const config = detail?.config
    return {
      issuer: config?.issuer ?? '',
      clientId: config?.clientId ?? '',
      clientSecret: config?.clientSecret ?? '',
      redirectUrl: config?.redirectUrl ?? getDefaultRedirectUrl(),
      scopes: config?.scopes?.join(' ') || DEFAULT_SCOPES,
      enabled: detail?.provider.enabled ?? provider?.enabled ?? false,
      allowRegistration: detail?.provider.allowRegistration ?? provider?.allowRegistration ?? false,
    }
  }, [detailsQuery.data, provider])

  const {
    control,
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<OIDCConfigFormValues>({
    resolver: zodResolver(oidcConfigSchema),
    defaultValues,
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const mutation = useMutation({
    mutationFn: authProvidersApi.configureOIDC,
    onSuccess: async () => {
      setFormError(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey('oidc'),
        }),
      ])
      toast.success('OIDC provider saved')
      onSuccess?.()
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      setFormError(apiError)
      toast.error('Failed to save OIDC provider', {
        description: apiError.message,
      })
    },
  })

  const onSubmit = handleSubmit(async (values) => {
    const scopes = values.scopes
      .split(/\s+/)
      .map((scope) => scope.trim())
      .filter(Boolean)

    await mutation.mutateAsync({
      enabled: values.enabled,
      allowRegistration: values.allowRegistration,
      config: {
        issuer: values.issuer.trim(),
        clientId: values.clientId.trim(),
        clientSecret: values.clientSecret.trim(),
        redirectUrl: values.redirectUrl.trim(),
        scopes,
      },
    })
  })

  const isSaving = isSubmitting || mutation.isPending

  return (
    <form className="space-y-6" onSubmit={onSubmit}>
      <div className="grid grid-cols-1 gap-4">
        <Input
          label="Issuer URL"
          placeholder="https://accounts.example.com"
          {...register('issuer')}
          error={errors.issuer?.message}
        />

        <Input
          label="Client ID"
          placeholder="oidc-client-id"
          {...register('clientId')}
          error={errors.clientId?.message}
        />

        <Input
          type="password"
          label="Client Secret"
          placeholder="••••••••"
          {...register('clientSecret')}
          error={errors.clientSecret?.message}
          helpText="This secret is stored encrypted; updating it will overwrite the existing value."
        />

        <Input
          label="Redirect URL"
          placeholder={getDefaultRedirectUrl()}
          {...register('redirectUrl')}
          error={errors.redirectUrl?.message}
          helpText="Register this callback URL with your identity provider."
        />

        <Input
          label="Scopes"
          placeholder={DEFAULT_SCOPES}
          {...register('scopes')}
          error={errors.scopes?.message}
          helpText="Space-separated scopes requested during authorization."
        />
      </div>

      <div className="space-y-3">
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
                <p className="text-sm font-medium text-foreground">Allow auto-provisioning</p>
                <p className="text-sm text-muted-foreground">
                  Automatically create user accounts for identities that authenticate via OIDC.
                </p>
              </div>
            </label>
          )}
        />

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
                <p className="text-sm font-medium text-foreground">Enable provider</p>
                <p className="text-sm text-muted-foreground">
                  When enabled, users can authenticate using this OIDC configuration.
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

      {detailsQuery.isError && !provider ? (
        <div className="rounded border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
          Failed to load existing configuration. You can still provide new settings below.
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onCancel} disabled={isSaving}>
          Cancel
        </Button>
        <Button type="submit" loading={isSaving}>
          Save Configuration
        </Button>
      </div>
    </form>
  )
}
