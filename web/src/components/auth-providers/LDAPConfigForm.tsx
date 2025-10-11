import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { ldapConfigSchema } from '@/schemas/authProviders'
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

type LDAPConfigFormValues = z.infer<typeof ldapConfigSchema>

interface LDAPConfigFormProps {
  provider?: AuthProviderRecord
  onCancel?: () => void
  onSuccess?: () => void
}

export function LDAPConfigForm({ provider, onCancel, onSuccess }: LDAPConfigFormProps) {
  const queryClient = useQueryClient()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const detailsQuery = useAuthProviderDetails('ldap', {
    enabled: Boolean(provider),
  })

  const defaultValues = useMemo<LDAPConfigFormValues>(() => {
    const detail = detailsQuery.data
    const config = detail?.config
    return {
      host: config?.host ?? '',
      port: config?.port ?? 389,
      baseDn: config?.userBaseDn ?? config?.baseDn ?? '',
      bindDn: config?.bindDn ?? '',
      bindPassword: config?.bindPassword ?? '',
      userFilter: config?.userFilter ?? '(uid={username})',
      useTls: config?.useTls ?? false,
      skipVerify: config?.skipVerify ?? false,
      attributeMapping: serializeAttributeMapping(config?.attributeMapping) || 'email=mail',
      syncGroups: config?.syncGroups ?? false,
      groupBaseDn: config?.groupBaseDn ?? '',
      groupNameAttribute: config?.groupNameAttribute ?? 'cn',
      groupMemberAttribute: config?.groupMemberAttribute ?? 'member',
      groupFilter: config?.groupFilter ?? '(objectClass=nestedGroup)',
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
    watch,
  } = useForm<LDAPConfigFormValues>({
    resolver: zodResolver(ldapConfigSchema) as Resolver<LDAPConfigFormValues>,
    defaultValues,
  })

  const syncGroupsEnabled = watch('syncGroups')

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const mutation = useMutation({
    mutationFn: authProvidersApi.configureLDAP,
    onSuccess: async () => {
      setFormError(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey('ldap'),
        }),
      ])
      toast.success('LDAP provider saved')
      onSuccess?.()
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      setFormError(apiError)
      toast.error('Failed to save LDAP provider', {
        description: apiError.message,
      })
    },
  })

  const onSubmit = handleSubmit(async (values) => {
    let attributeMapping: Record<string, string> = {}
    if (values.attributeMapping?.trim()) {
      try {
        attributeMapping = parseAttributeMapping(values.attributeMapping)
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
        host: values.host.trim(),
        port: values.port,
        baseDn: values.baseDn.trim(),
        userBaseDn: values.baseDn.trim(),
        bindDn: values.bindDn.trim(),
        bindPassword: values.bindPassword.trim(),
        userFilter: values.userFilter.trim(),
        useTls: values.useTls,
        skipVerify: values.skipVerify,
        attributeMapping,
        syncGroups: values.syncGroups,
        groupBaseDn: values.groupBaseDn?.trim() ?? '',
        groupNameAttribute: values.groupNameAttribute?.trim() ?? '',
        groupMemberAttribute: values.groupMemberAttribute?.trim() ?? '',
        groupFilter: values.groupFilter?.trim() ?? '',
      },
    })
  })

  const isSaving = isSubmitting || mutation.isPending

  return (
    <form className="space-y-6" onSubmit={onSubmit}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="Host"
          placeholder="ldap.example.com"
          {...register('host')}
          error={errors.host?.message}
        />
        <Input
          type="number"
          label="Port"
          min={1}
          placeholder="389"
          {...register('port', { valueAsNumber: true })}
          error={errors.port?.message}
        />
        <Input
          label="LDAP Users DN"
          placeholder="dc=example,dc=com"
          {...register('baseDn')}
          error={errors.baseDn?.message}
        />
        <Input
          label="Bind DN"
          placeholder="cn=admin,dc=example,dc=com"
          {...register('bindDn')}
          error={errors.bindDn?.message}
        />
        <Input
          type="password"
          label="Bind Password"
          placeholder="••••••••"
          {...register('bindPassword')}
          error={errors.bindPassword?.message}
        />
        <Input
          label="User Filter"
          placeholder="(uid={username})"
          {...register('userFilter')}
          error={errors.userFilter?.message}
        />
      </div>

      <Textarea
        label="Attribute Mapping"
        placeholder={'email=mail\nusername=uid'}
        {...register('attributeMapping')}
        error={errors.attributeMapping?.message}
        helpText="One mapping per line in key=value format."
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="LDAP Groups DN"
          placeholder="ou=Groups,dc=example,dc=com"
          {...register('groupBaseDn')}
          error={errors.groupBaseDn?.message}
          disabled={!syncGroupsEnabled}
        />
        <Input
          label="Group Filter"
          placeholder="(objectClass=nestedGroup)"
          {...register('groupFilter')}
          error={errors.groupFilter?.message}
          disabled={!syncGroupsEnabled}
        />
        <Input
          label="Group Name LDAP Attribute"
          placeholder="cn"
          {...register('groupNameAttribute')}
          error={errors.groupNameAttribute?.message}
          disabled={!syncGroupsEnabled}
        />
        <Input
          label="Membership LDAP Attribute"
          placeholder="member"
          {...register('groupMemberAttribute')}
          error={errors.groupMemberAttribute?.message}
          disabled={!syncGroupsEnabled}
        />
      </div>

      <div className="space-y-3">
        <Controller
          control={control}
          name="useTls"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Use TLS</p>
                <p className="text-sm text-muted-foreground">
                  Establish LDAPS connections (recommended for production deployments).
                </p>
              </div>
            </label>
          )}
        />

        <Controller
          control={control}
          name="skipVerify"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Skip certificate verification</p>
                <p className="text-sm text-muted-foreground">
                  Ignore TLS certificate validation (useful for lab environments).
                </p>
              </div>
            </label>
          )}
        />

        <Controller
          control={control}
          name="syncGroups"
          render={({ field }) => (
            <label className="flex items-start gap-3">
              <Checkbox
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(Boolean(checked))}
              />
              <div>
                <p className="text-sm font-medium text-foreground">Sync groups to teams</p>
                <p className="text-sm text-muted-foreground">
                  When enabled, LDAP group memberships will be mirrored to ShellCN teams during
                  login and manual syncs.
                </p>
              </div>
            </label>
          )}
        />

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
                  Automatically create ShellCN accounts for LDAP users on first sign-in.
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
                  When enabled, users can authenticate using this LDAP configuration.
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
