import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { Controller, type Control, useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { Skeleton } from '@/components/ui/Skeleton'
import { useTeams } from '@/hooks/useTeams'
import { useCredentialTemplates, useIdentity, useIdentityMutations } from '@/hooks/useIdentities'
import type {
  CredentialField,
  CredentialTemplateRecord,
  IdentityRecord,
  IdentityScope,
} from '@/types/vault'
import { cn } from '@/lib/utils/cn'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/Select'

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

type IdentityFormValues = z.infer<typeof identityFormSchema>

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
  existing?: Record<string, unknown>
): Record<string, unknown> {
  if (!template) {
    return existing ?? {}
  }
  const defaults: Record<string, unknown> = {}
  template.fields.forEach((field) => {
    if (existing && field.name in existing) {
      defaults[field.name] = existing[field.name]
      return
    }
    if (field.default_value !== undefined) {
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
        payload: buildTemplateDefaults(template, identity.payload ?? {}),
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
        const rawValue = currentPayload[field.name]
        if (rawValue === undefined || rawValue === '' || rawValue === null) {
          if (field.required && mode === 'create') {
            payload[field.name] = ''
          }
          return
        }
        switch (field.type) {
          case 'number':
            payload[field.name] = Number(rawValue)
            break
          case 'boolean':
            payload[field.name] = Boolean(rawValue)
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
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label htmlFor="identity-name" className="text-sm font-medium">
                Name
              </label>
              <Input id="identity-name" placeholder="Production SSH key" {...register('name')} />
              {formState.errors.name ? (
                <p className="text-xs text-destructive">{formState.errors.name.message}</p>
              ) : null}
            </div>
            <div className="space-y-2">
              <label htmlFor="identity-scope" className="text-sm font-medium">
                Scope
              </label>
              <Controller
                name="scope"
                control={control}
                render={({ field }) => (
                  <Select
                    disabled={mode === 'edit'}
                    value={(field.value as IdentityScope) ?? defaultScope}
                    onValueChange={(value) => field.onChange(value as IdentityScope)}
                  >
                    <SelectTrigger id="identity-scope" className="h-10 w-full justify-between">
                      <SelectValue placeholder="Select scope" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="global">Global</SelectItem>
                      <SelectItem value="team">Team</SelectItem>
                      <SelectItem value="connection">Connection</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
          </div>

          <div className="space-y-2">
            <label htmlFor="identity-description" className="text-sm font-medium">
              Description
            </label>
            <Textarea
              id="identity-description"
              rows={3}
              placeholder="Optional summary to help collaborators understand this credential"
              {...register('description')}
            />
            {formState.errors.description ? (
              <p className="text-xs text-destructive">{formState.errors.description.message}</p>
            ) : null}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label htmlFor="identity-template" className="text-sm font-medium">
                Credential template
              </label>
              <Controller
                name="template_id"
                control={control}
                render={({ field }) => (
                  <Select value={field.value ?? ''} onValueChange={field.onChange}>
                    <SelectTrigger id="identity-template" className="h-10 w-full justify-between">
                      <SelectValue placeholder="Custom (JSON)" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="">Custom (JSON)</SelectItem>
                      {templates.map((tpl) => (
                        <SelectItem key={tpl.id} value={tpl.id}>
                          {tpl.display_name} · {tpl.driver_id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="identity-team" className="text-sm font-medium">
                Team (team scope)
              </label>
              <Controller
                name="team_id"
                control={control}
                render={({ field }) => (
                  <Select
                    disabled={disableTeamSelect}
                    value={field.value ?? ''}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger
                      id="identity-team"
                      className={cn(
                        'h-10 w-full justify-between',
                        disableTeamSelect && 'bg-muted text-muted-foreground'
                      )}
                    >
                      <SelectValue placeholder="No team" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="">No team</SelectItem>
                      {teams.map((team) => (
                        <SelectItem key={team.id} value={team.id}>
                          {team.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {disableTeamSelect ? (
                <p className="text-xs text-muted-foreground">
                  Switch to the Team scope to select a default team.
                </p>
              ) : null}
            </div>
          </div>

          <div className="space-y-2">
            <label htmlFor="identity-metadata" className="text-sm font-medium">
              Metadata (JSON)
            </label>
            <Textarea
              id="identity-metadata"
              rows={4}
              placeholder='{"notes":"Optional metadata"}'
              {...register('metadata')}
            />
            {formState.errors.metadata ? (
              <p className="text-xs text-destructive">{formState.errors.metadata.message}</p>
            ) : null}
          </div>

          {selectedTemplate ? (
            <div className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                    Credential fields
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    Provide values that match the selected template. Sensitive values remain
                    encrypted.
                  </p>
                </div>
                {mode === 'edit' ? (
                  <label className="flex items-center gap-2 text-sm text-foreground">
                    <Controller
                      name="rotate_payload"
                      control={control}
                      render={({ field }) => (
                        <Checkbox
                          checked={Boolean(field.value)}
                          onCheckedChange={(checked) => field.onChange(Boolean(checked))}
                        />
                      )}
                    />
                    Rotate credentials
                  </label>
                ) : null}
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                {selectedTemplate.fields.map((field) => (
                  <CredentialFieldInput
                    key={field.name}
                    field={field}
                    control={control}
                    disabled={mode === 'edit' && !rotatePayload}
                  />
                ))}
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              <label htmlFor="identity-custom-payload" className="text-sm font-medium">
                Credential payload (JSON)
              </label>
              <Textarea
                id="identity-custom-payload"
                rows={6}
                placeholder='{"username":"admin","password":"••••"}'
                {...register('customPayload')}
              />
            </div>
          )}

          {formError ? <p className="text-sm text-destructive">{formError}</p> : null}

          <div className="flex justify-end gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={handleModalClose}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Saving…' : mode === 'create' ? 'Create identity' : 'Save changes'}
            </Button>
          </div>
        </form>
      )}
    </Modal>
  )
}

interface CredentialFieldInputProps {
  field: CredentialField
  control: Control<IdentityFormValues>
  disabled?: boolean
}

function CredentialFieldInput({ field, control, disabled }: CredentialFieldInputProps) {
  const fieldName = `payload.${field.name}` as const

  if (field.type === 'boolean') {
    return (
      <Controller
        name={fieldName}
        control={control}
        defaultValue={false}
        render={({ field: controllerField }) => (
          <label
            className={cn(
              'flex items-start gap-3 rounded-md border border-border p-3',
              disabled ? 'bg-muted text-muted-foreground' : ''
            )}
          >
            <Checkbox
              checked={Boolean(controllerField.value)}
              onCheckedChange={(checked) => controllerField.onChange(Boolean(checked))}
              disabled={disabled}
            />
            <div>
              <p className="text-sm font-medium text-foreground">{field.label ?? field.name}</p>
              {field.description ? (
                <p className="text-xs text-muted-foreground">{field.description}</p>
              ) : null}
            </div>
          </label>
        )}
      />
    )
  }

  if (field.type === 'enum' && Array.isArray(field.options)) {
    return (
      <div className="space-y-2">
        <label htmlFor={`field-${field.name}`} className="text-sm font-medium">
          {field.label ?? field.name}
        </label>
        <Controller
          name={fieldName}
          control={control}
          defaultValue={
            typeof field.options[0] === 'string'
              ? field.options[0]
              : field.options[0] && 'value' in field.options[0]
                ? String(field.options[0].value ?? '')
                : ''
          }
          render={({ field: controllerField }) => (
            <Select
              disabled={disabled}
              value={(controllerField.value as string) ?? ''}
              onValueChange={(value) => controllerField.onChange(value)}
            >
              <SelectTrigger id={`field-${field.name}`} className="h-10 w-full justify-between">
                <SelectValue placeholder="Select option" />
              </SelectTrigger>
              <SelectContent align="start">
                {field.options?.map((option) => {
                  if (typeof option === 'string') {
                    return (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    )
                  }
                  const rawValue = option.value
                  const label =
                    typeof option.label === 'string' ? option.label : String(option.value ?? '')
                  return (
                    <SelectItem key={String(rawValue)} value={String(rawValue)}>
                      {label}
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          )}
        />
        {field.description ? (
          <p className="text-xs text-muted-foreground">{field.description}</p>
        ) : null}
      </div>
    )
  }

  const isSecret = field.type === 'secret'
  const InputComponent = isSecret ? Textarea : Input
  const inputProps = isSecret ? { rows: field.input_modes?.includes('textarea') ? 4 : 3 } : {}

  return (
    <div className="space-y-2">
      <label htmlFor={`field-${field.name}`} className="text-sm font-medium">
        {field.label ?? field.name}
      </label>
      <Controller
        name={fieldName}
        control={control}
        defaultValue={field.default_value ?? ''}
        render={({ field: controllerField }) => (
          <InputComponent
            id={`field-${field.name}`}
            disabled={disabled}
            placeholder={field.description ?? ''}
            value={(controllerField.value as string) ?? ''}
            onChange={(event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
              controllerField.onChange(event.target.value)
            }
            {...inputProps}
          />
        )}
      />
      {field.description ? (
        <p className="text-xs text-muted-foreground">{field.description}</p>
      ) : null}
    </div>
  )
}
