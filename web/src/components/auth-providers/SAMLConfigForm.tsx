import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { samlConfigSchema } from '@/schemas/authProviders'
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
import { parseAttributeMapping, serializeAttributeMapping } from '@/lib/utils/auth-providers'

type SAMLConfigFormValues = z.infer<typeof samlConfigSchema>

interface SAMLConfigFormProps {
  provider?: AuthProviderRecord
  onCancel?: () => void
  onSuccess?: () => void
}

export function SAMLConfigForm({ provider, onCancel, onSuccess }: SAMLConfigFormProps) {
  const queryClient = useQueryClient()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const detailsQuery = useAuthProviderDetails('saml', {
    enabled: Boolean(provider),
  })

  const defaultValues = useMemo<SAMLConfigFormValues>(() => {
    const detail = detailsQuery.data
    const config = detail?.config
    return {
      metadataUrl: config?.metadataUrl ?? '',
      entityId: config?.entityId ?? '',
      ssoUrl: config?.ssoUrl ?? '',
      acsUrl: config?.acsUrl ?? '',
      certificate: config?.certificate ?? '',
      privateKey: config?.privateKey ?? '',
      attributeMapping: serializeAttributeMapping(config?.attributeMapping) || 'email=mail',
      enabled: detail?.provider.enabled ?? provider?.enabled ?? false,
      allowRegistration: detail?.provider.allowRegistration ?? provider?.allowRegistration ?? false,
    }
  }, [detailsQuery.data, provider])

  const {
    control,
    register,
    setError,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<SAMLConfigFormValues>({
    resolver: zodResolver(samlConfigSchema) as Resolver<SAMLConfigFormValues>,
    defaultValues,
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const mutation = useMutation({
    mutationFn: authProvidersApi.configureSAML,
    onSuccess: async () => {
      setFormError(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey('saml'),
        }),
      ])
      toast.success('SAML provider saved')
      onSuccess?.()
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      setFormError(apiError)
      toast.error('Failed to save SAML provider', {
        description: apiError.message,
      })
    },
  })

  const onSubmit = handleSubmit(async (values) => {
    let attributeMapping: Record<string, string> = {}
    const mappingText = values.attributeMapping ?? ''
    if (mappingText.trim()) {
      try {
        attributeMapping = parseAttributeMapping(mappingText)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Invalid attribute mapping'
        setError('attributeMapping', { type: 'manual', message })
        return
      }
    }

    await mutation.mutateAsync({
      enabled: values.enabled,
      allowRegistration: values.allowRegistration,
      config: {
        metadataUrl: values.metadataUrl?.trim() || undefined,
        entityId: values.entityId.trim(),
        ssoUrl: values.ssoUrl.trim(),
        acsUrl: values.acsUrl.trim(),
        certificate: values.certificate.trim(),
        privateKey: values.privateKey.trim(),
        attributeMapping,
      },
    })
  })

  const isSaving = isSubmitting || mutation.isPending

  return (
    <form className="space-y-6" onSubmit={onSubmit}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="Entity ID"
          placeholder="https://shellcn.example.com/saml/metadata"
          {...register('entityId')}
          error={errors.entityId?.message}
        />
        <Input
          label="Metadata URL"
          placeholder="https://idp.example.com/metadata.xml"
          {...register('metadataUrl')}
          error={errors.metadataUrl?.message}
          helpText="Optional: automatically fetch and refresh metadata from this URL."
        />
        <Input
          label="SSO URL"
          placeholder="https://idp.example.com/sso"
          {...register('ssoUrl')}
          error={errors.ssoUrl?.message}
        />
        <Input
          label="ACS URL"
          placeholder="https://shellcn.example.com/api/auth/providers/saml/callback"
          {...register('acsUrl')}
          error={errors.acsUrl?.message}
        />
      </div>

      <Textarea
        label="Certificate (PEM)"
        placeholder="-----BEGIN CERTIFICATE-----"
        {...register('certificate')}
        error={errors.certificate?.message}
        helpText="Paste the IdP signing certificate in PEM format."
      />

      <Textarea
        label="Private Key (PEM)"
        placeholder="-----BEGIN PRIVATE KEY-----"
        {...register('privateKey')}
        error={errors.privateKey?.message}
        helpText="Used for signing assertions. It will be encrypted at rest."
      />

      <Textarea
        label="Attribute Mapping"
        placeholder={'email=mail\nfirst_name=givenName\nlast_name=sn'}
        {...register('attributeMapping')}
        error={errors.attributeMapping?.message}
        helpText="One mapping per line in key=value format."
      />

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
                  Automatically create user accounts the first time they authenticate via SAML.
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
                  When enabled, users can sign in using this SAML configuration.
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

      {detailsQuery.isError && provider ? (
        <div className="rounded border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
          Failed to load the existing configuration. Saving will overwrite stored values.
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
