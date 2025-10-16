import { useEffect, useMemo, useState } from 'react'
import { useForm, type FieldErrors } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Modal } from '@/components/ui/Modal'
import { Skeleton } from '@/components/ui/Skeleton'
import { useTeams } from '@/hooks/useTeams'
import { useCredentialTemplates, useIdentity, useIdentityMutations } from '@/hooks/useIdentities'
import type { CredentialTemplateRecord, IdentityRecord, IdentityScope } from '@/types/vault'
import { isCredentialFieldVisible } from '@/lib/vault/credentialFieldVisibility'
import {
  CustomPayloadSection,
  FormActions,
  IdentityDetailsSection,
  MetadataSection,
  TemplateFieldsSection,
  TemplateSelectionSection,
} from './identity-form/Sections'

const identityFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(2, 'Name must contain at least 2 characters')
    .max(128, 'Name cannot exceed 128 characters'),
  description: z
    .string()
    .trim()
    .max(512, 'Description cannot exceed 512 characters')
    .optional()
    .or(z.literal('')),
  scope: z.enum(['global', 'team', 'connection']),
  template_id: z.string().trim().optional().or(z.literal('')),
  team_id: z.string().trim().optional().or(z.literal('')),
  metadata: z.string().optional().or(z.literal('')),
  rotate_payload: z.boolean().optional(),
  customPayload: z.string().optional().or(z.literal('')),
  payload: z.record(z.string(), z.any()).optional(),
})

const MASKED_SECRET_PLACEHOLDER = '••••••••'

export type IdentityFormValues = z.infer<typeof identityFormSchema>
export type IdentityFormErrors = FieldErrors<IdentityFormValues>

interface IdentityFormModalProps {
  open: boolean
  onClose: () => void
  mode: 'create' | 'edit'
  identityId?: string
  defaultScope?: IdentityScope
  connectionId?: string | null
  onSuccess?: (identity: IdentityRecord) => void
}

function resolveTemplate(
  templateId: string | undefined,
  templates: CredentialTemplateRecord[]
): CredentialTemplateRecord | undefined {
  if (!templateId) {
    return undefined
  }
  return templates.find((tpl) => tpl.id === templateId)
}

function toMetadataObject(raw?: string | null): Record<string, unknown> | undefined {
  if (!raw) {
    return undefined
  }
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return undefined
  }
  return undefined
}

function stringifyMetadata(value?: Record<string, unknown> | null): string {
  if (!value || Object.keys(value).length === 0) {
    return ''
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return ''
  }
}

function buildTemplateDefaults(
  template: CredentialTemplateRecord | undefined,
  existing?: Record<string, unknown>,
  options?: { maskSecrets?: boolean }
): Record<string, unknown> {
  if (!template) {
    return existing ?? {}
  }
  const defaults: Record<string, unknown> = {}
  template.fields.forEach((field) => {
    if (existing && field.name in existing) {
      const existingValue = existing[field.name]
      if (
        options?.maskSecrets &&
        field.type === 'secret' &&
        existingValue !== undefined &&
        existingValue !== null &&
        String(existingValue).length > 0
      ) {
        defaults[field.name] = MASKED_SECRET_PLACEHOLDER
        return
      }
      defaults[field.name] = existingValue
      return
    }
    if (field.default_value !== undefined && !hasDefinedValue(defaults[field.name])) {
      defaults[field.name] = field.default_value
      return
    }
    switch (field.type) {
      case 'boolean':
        defaults[field.name] = false
        break
      default:
        defaults[field.name] = ''
    }
  })

  const templateMetadata = toRecord(template.metadata)
  if (templateMetadata) {
    const metadataDefaults = toRecord(templateMetadata.defaults)
    if (metadataDefaults) {
      Object.entries(metadataDefaults).forEach(([key, value]) => {
        if (!hasDefinedValue(defaults[key])) {
          defaults[key] = value
        }
      })
    }

    template.fields.forEach((field) => {
      const metadataKey = `default_${field.name}`
      if (metadataKey in templateMetadata && !hasDefinedValue(defaults[field.name])) {
        defaults[field.name] = templateMetadata[metadataKey]
      }
    })
  }

  return defaults
}

export function IdentityFormModal({
  open,
  onClose,
  mode,
  identityId,
  defaultScope = 'global',
  connectionId,
  onSuccess,
}: IdentityFormModalProps) {
  const templatesQuery = useCredentialTemplates({ enabled: open })
  const templates = useMemo(() => templatesQuery.data ?? [], [templatesQuery.data])
  const teamsQuery = useTeams({ enabled: open })
  const teams = useMemo(() => teamsQuery.data?.data ?? [], [teamsQuery.data])

  const identityQuery = useIdentity(identityId, {
    enabled: mode === 'edit' && Boolean(identityId) && open,
    includePayload: true,
  })

  const { create, update } = useIdentityMutations(identityId)
  const [formError, setFormError] = useState<string | null>(null)

  const defaultValues = useMemo<IdentityFormValues>(() => {
    if (mode === 'edit' && identityQuery.data) {
      const identity = identityQuery.data
      const template = resolveTemplate(identity.template_id ?? undefined, templates)
      return {
        name: identity.name,
        description: identity.description ?? '',
        scope: identity.scope,
        template_id: identity.template_id ?? '',
        team_id: identity.team_id ?? '',
        metadata: stringifyMetadata(identity.metadata ?? undefined),
        rotate_payload: false,
        customPayload: '',
        payload: buildTemplateDefaults(template, identity.payload ?? {}, { maskSecrets: true }),
      }
    }
    return {
      name: '',
      description: '',
      scope: defaultScope,
      template_id: '',
      team_id: '',
      metadata: '',
      rotate_payload: true,
      customPayload: '',
      payload: {},
    }
  }, [defaultScope, identityQuery.data, mode, templates])

  const { register, control, watch, handleSubmit, reset, setValue, formState } =
    useForm<IdentityFormValues>({
      resolver: zodResolver(identityFormSchema),
      defaultValues,
    })

  const selectedScope = watch('scope')
  const selectedTemplateId = watch('template_id')
  const rotatePayload = watch('rotate_payload') ?? mode === 'create'
  const selectedTemplate = useMemo(() => {
    const templateId = selectedTemplateId || identityQuery.data?.template_id || ''
    return resolveTemplate(templateId || undefined, templates)
  }, [identityQuery.data?.template_id, selectedTemplateId, templates])

  useEffect(() => {
    if (open) {
      setFormError(null)
      reset(defaultValues)
    }
  }, [defaultValues, open, reset])

  useEffect(() => {
    if (selectedScope !== 'team') {
      setValue('team_id', '')
    }
  }, [selectedScope, setValue])

  useEffect(() => {
    if (!selectedTemplateId) {
      return
    }
    const template = resolveTemplate(selectedTemplateId, templates)
    if (template) {
      setValue('customPayload', '')
      setValue('payload', buildTemplateDefaults(template))
    }
  }, [selectedTemplateId, setValue, templates])

  useEffect(() => {
    if (!open || mode !== 'edit' || !selectedTemplate) {
      return
    }
    if (rotatePayload) {
      selectedTemplate.fields
        .filter((field) => field.type === 'secret')
        .forEach((field) => setValue(`payload.${field.name}` as const, ''))
      return
    }
    if (!identityQuery.data?.payload) {
      return
    }
    selectedTemplate.fields
      .filter((field) => field.type === 'secret')
      .forEach((field) => {
        if (identityQuery.data?.payload && field.name in identityQuery.data.payload) {
          setValue(`payload.${field.name}` as const, MASKED_SECRET_PLACEHOLDER)
        }
      })
  }, [identityQuery.data, mode, open, rotatePayload, selectedTemplate, setValue])

  const handleModalClose = () => {
    if (create.isPending || update.isPending) {
      return
    }
    onClose()
  }

  const isSubmitting = create.isPending || update.isPending
  const disableTeamSelect = selectedScope !== 'team'

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)

    const metadata = values.metadata ? toMetadataObject(values.metadata) : undefined
    if (values.metadata && !metadata) {
      setFormError('Metadata must be valid JSON object')
      return
    }

    let payload: Record<string, unknown> = {}
    if (selectedTemplate) {
      const currentPayload = values.payload ?? {}
      selectedTemplate.fields.forEach((field) => {
        if (!isCredentialFieldVisible(field, currentPayload)) {
          return
        }
        const rawValue = currentPayload[field.name]
        if (rawValue === undefined || rawValue === '' || rawValue === null) {
          if (field.required && mode === 'create') {
            payload[field.name] = ''
          }
          return
        }
        if (rawValue === MASKED_SECRET_PLACEHOLDER) {
          return
        }
        switch (field.type) {
          case 'number':
            {
              const numericValue =
                typeof rawValue === 'number' ? rawValue : Number(String(rawValue).trim())
              if (!Number.isNaN(numericValue)) {
                payload[field.name] = numericValue
              }
            }
            break
          case 'boolean':
            payload[field.name] = coerceBoolean(rawValue)
            break
          default:
            payload[field.name] = rawValue
        }
      })
    } else if (values.customPayload) {
      const parsed = toMetadataObject(values.customPayload)
      if (!parsed) {
        setFormError('Credential payload must be valid JSON when no template is selected')
        return
      }
      payload = parsed
    }

    try {
      if (mode === 'create') {
        const identity = await create.mutateAsync({
          name: values.name,
          description: values.description || undefined,
          scope: values.scope,
          template_id: values.template_id || undefined,
          team_id: values.scope === 'team' ? values.team_id || undefined : undefined,
          connection_id: values.scope === 'connection' ? (connectionId ?? undefined) : undefined,
          metadata,
          payload,
        })
        onSuccess?.(identity)
        handleModalClose()
      } else if (identityId) {
        const updatePayload: Record<string, unknown> = {
          name: values.name,
          description: values.description || undefined,
          template_id: values.template_id || undefined,
          metadata,
        }
        if (values.scope === 'team') {
          updatePayload.team_id = values.team_id || undefined
        }
        if (values.scope === 'connection') {
          updatePayload.connection_id = identityQuery.data?.connection_id ?? undefined
        }
        if (rotatePayload) {
          updatePayload.payload = payload
        }
        const identity = await update.mutateAsync({ identityId, payload: updatePayload })
        onSuccess?.(identity)
        handleModalClose()
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to save identity'
      setFormError(message)
    }
  })

  return (
    <Modal
      open={open}
      onClose={handleModalClose}
      size="2xl"
      title={mode === 'create' ? 'Create Identity' : 'Edit Identity'}
      description="Securely store and manage credentials for connections."
    >
      {mode === 'edit' && identityQuery.isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : (
        <form className="space-y-6" onSubmit={onSubmit}>
          <IdentityDetailsSection
            register={register}
            control={control}
            errors={formState.errors}
            defaultScope={defaultScope}
            mode={mode}
          />
          <TemplateSelectionSection
            control={control}
            templates={templates}
            teams={teams}
            disableTeamSelect={disableTeamSelect}
            selectedTemplate={selectedTemplate}
          />
          {selectedTemplate ? (
            <TemplateFieldsSection
              control={control}
              template={selectedTemplate}
              mode={mode}
              rotatePayload={rotatePayload}
            />
          ) : (
            <CustomPayloadSection register={register} />
          )}
          <MetadataSection register={register} error={formState.errors.metadata?.message} />
          <FormActions
            onCancel={handleModalClose}
            isSubmitting={isSubmitting}
            mode={mode}
            formError={formError}
          />
        </form>
      )}
    </Modal>
  )
}

function hasDefinedValue(value: unknown): boolean {
  return value !== undefined && value !== null && value !== ''
}

function coerceBoolean(value: unknown): boolean {
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['true', '1', 'yes', 'y', 'on'].includes(normalized)) {
      return true
    }
    if (['false', '0', 'no', 'n', 'off'].includes(normalized)) {
      return false
    }
  }
  if (typeof value === 'number') {
    return value !== 0
  }
  return Boolean(value)
}

function toRecord(value: unknown): Record<string, unknown> | undefined {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return undefined
}
