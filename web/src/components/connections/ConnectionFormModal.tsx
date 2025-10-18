import { useEffect, useMemo, useRef, useState } from 'react'
import { z } from 'zod'
import { Controller, useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { IdentitySelector } from '@/components/vault/IdentitySelector'
import { IdentityFormModal } from '@/components/vault/IdentityFormModal'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { resolveProtocolIcon } from '@/lib/utils/protocolIcons'
import { useConnectionMutations } from '@/hooks/useConnectionMutations'
import { ApiError, toApiError } from '@/lib/api/http'
import { teamsApi } from '@/lib/api/teams'
import type { ConnectionCreatePayload } from '@/lib/api/connections'
import { usePermissions } from '@/hooks/usePermissions'
import {
  CONNECTION_COLOR_OPTIONS,
  CONNECTION_ICON_OPTIONS,
  DEFAULT_CONNECTION_ICON_ID,
  getDefaultIconForProtocol,
  getIconOptionsForProtocol,
} from '@/constants/connections'
import { PERMISSIONS } from '@/constants/permissions'
import { cn } from '@/lib/utils/cn'
import type { Protocol } from '@/types/protocols'
import type {
  ConnectionFolderNode,
  ConnectionRecord,
  ConnectionTemplateMetadata,
} from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { useConnectionTemplate } from '@/hooks/useConnectionTemplate'
import { ConnectionTemplateForm } from '@/components/connections/ConnectionTemplateForm'
import {
  isFieldVisible,
  type TemplateValueMap,
} from '@/components/connections/connectionTemplateHelpers'
import type { ConnectionTemplate } from '@/types/protocols'

const connectionSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Connection name is required')
    .max(255, 'Connection name must be 255 characters or less'),
  description: z
    .string()
    .trim()
    .max(1000, 'Description must be 1000 characters or less')
    .optional()
    .or(z.literal('')),
  folder_id: z.string().trim().optional().or(z.literal('')),
  team_id: z.string().trim().optional().or(z.literal('')),
  icon: z.string().trim().optional().or(z.literal('')),
  color: z.string().trim().optional().or(z.literal('')),
})

type ConnectionFormValues = z.infer<typeof connectionSchema>

interface ConnectionFormModalProps {
  open: boolean
  onClose: () => void
  protocol: Protocol | null
  folders: ConnectionFolderNode[]
  teamId?: string | null
  teams?: TeamRecord[]
  allowTeamAssignment?: boolean
  onSuccess: (connection: ConnectionRecord) => void
  mode?: 'create' | 'edit'
  connection?: ConnectionRecord | null
}

export function ConnectionFormModal({
  open,
  onClose,
  protocol,
  folders,
  teamId,
  teams = [],
  allowTeamAssignment = false,
  onSuccess,
  mode = 'create',
  connection = null,
}: ConnectionFormModalProps) {
  const { create, update } = useConnectionMutations()
  const { hasPermission } = usePermissions()
  const canCreateIdentity = hasPermission(PERMISSIONS.VAULT.CREATE)
  const [formError, setFormError] = useState<ApiError | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [fieldValues, setFieldValues] = useState<TemplateValueMap>({})
  const [selectedIdentityId, setSelectedIdentityId] = useState<string | null>(null)
  const [identityModalOpen, setIdentityModalOpen] = useState(false)
  const [autoGrantTeamPermissions, setAutoGrantTeamPermissions] = useState(false)
  const grantToggleInteractedRef = useRef(false)
  const formInitializedRef = useRef(false)

  const { data: template, isLoading: templateLoading } = useConnectionTemplate(protocol?.id)

  const iconOptions = useMemo(() => {
    return getIconOptionsForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const defaultIcon = useMemo(() => {
    return getDefaultIconForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const {
    register,
    control,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<ConnectionFormValues>({
    resolver: zodResolver(connectionSchema),
    defaultValues: {
      name: '',
      description: '',
      folder_id: '',
      team_id: normalizeTeamValue(teamId),
      icon: defaultIcon ?? DEFAULT_CONNECTION_ICON_ID,
      color: '',
    },
  })

  const templateMetadata = useMemo<ConnectionTemplateMetadata | undefined>(
    () => extractTemplateMetadata(connection?.metadata),
    [connection?.metadata]
  )

  const initialFieldValues = useMemo(() => {
    if (!template) {
      return {}
    }
    return buildInitialFieldValues(template, templateMetadata?.fields ?? {})
  }, [template, templateMetadata?.fields])

  useEffect(() => {
    if (!open) {
      setFormError(null)
      setFieldErrors({})
      setFieldValues({})
      setSelectedIdentityId(null)
      setAutoGrantTeamPermissions(false)
      grantToggleInteractedRef.current = false
      formInitializedRef.current = false
      return
    }
    if (!protocol) {
      return
    }
    if (templateLoading) {
      return
    }
    if (formInitializedRef.current) {
      return
    }

    const metadata = (connection?.metadata ?? {}) as Record<string, unknown>
    const iconFromMetadata =
      typeof metadata.icon === 'string' && metadata.icon.trim().length > 0
        ? metadata.icon.trim()
        : undefined
    const colorFromMetadata =
      typeof metadata.color === 'string' && metadata.color.trim().length > 0
        ? metadata.color.trim()
        : ''

    const initialValues: ConnectionFormValues = {
      name: connection?.name ?? '',
      description: connection?.description ?? '',
      folder_id: connection?.folder_id ?? '',
      team_id: normalizeTeamValue(connection?.team_id ?? teamId),
      icon: iconFromMetadata ?? defaultIcon ?? DEFAULT_CONNECTION_ICON_ID,
      color: colorFromMetadata,
    }

    reset(initialValues)
    setFieldValues(initialFieldValues)
    setFieldErrors({})
    setFormError(null)
    grantToggleInteractedRef.current = false
    setAutoGrantTeamPermissions(false)
    setSelectedIdentityId(connection?.identity_id ?? null)
    formInitializedRef.current = true
  }, [open, protocol, templateLoading, reset, connection, teamId, defaultIcon, initialFieldValues])

  const selectedIcon = watch('icon')
  const selectedColor = watch('color')
  const selectedTeamValue = watch('team_id')
  const selectedFolderId = watch('folder_id')

  const effectiveTeamId = useMemo(
    () => denormalizeTeamValue(selectedTeamValue),
    [selectedTeamValue]
  )

  const teamCapabilitiesQuery = useQuery({
    queryKey: effectiveTeamId
      ? ['teams', effectiveTeamId, 'capabilities']
      : ['teams', 'capabilities'],
    queryFn: () => teamsApi.capabilities(effectiveTeamId!),
    enabled: Boolean(effectiveTeamId) && open,
    staleTime: 60_000,
  })

  const selectedTeam = useMemo(() => {
    if (!effectiveTeamId) {
      return null
    }
    return teams.find((team) => team.id === effectiveTeamId) ?? null
  }, [effectiveTeamId, teams])

  const { missingPermissionIds: missingTeamPermissionIds, messages: teamCapabilityWarnings } =
    useMemo(() => {
      if (!effectiveTeamId) {
        return { missingPermissionIds: [], messages: [] as string[] }
      }
      const capabilities = teamCapabilitiesQuery.data
      if (!capabilities) {
        return { missingPermissionIds: [], messages: [] as string[] }
      }
      return extractTeamCapabilityWarnings(capabilities)
    }, [effectiveTeamId, teamCapabilitiesQuery.data])

  useEffect(() => {
    if (!open) {
      return
    }
    if (missingTeamPermissionIds.length === 0) {
      grantToggleInteractedRef.current = false
      setAutoGrantTeamPermissions(false)
    }
  }, [missingTeamPermissionIds.length, open])

  const requiresIdentity = useMemo(() => {
    if (protocol?.identityRequired !== undefined) {
      return protocol.identityRequired
    }
    return Boolean(template?.metadata?.requires_identity ?? templateMetadata?.requires_identity)
  }, [protocol?.identityRequired, template?.metadata, templateMetadata?.requires_identity])

  const handleAutoGrantToggle = (next: boolean) => {
    grantToggleInteractedRef.current = true
    setAutoGrantTeamPermissions(next)
  }

  const handleFieldChange = (key: string, value: unknown) => {
    setFieldValues((previous) => ({
      ...previous,
      [key]: value,
    }))
    setFieldErrors((previous) => {
      if (!previous[key]) {
        return previous
      }
      const rest = { ...previous }
      delete rest[key]
      return rest
    })
  }

  const onSubmit: SubmitHandler<ConnectionFormValues> = async (values) => {
    setFormError(null)
    setFieldErrors({})
    if (!protocol) {
      return
    }

    if (requiresIdentity && !selectedIdentityId) {
      setFormError(
        new ApiError({
          code: 'validation.identity_required',
          message: 'Select or create a vault identity to continue.',
        })
      )
      return
    }

    let sanitizedFields: Record<string, unknown> | undefined
    if (template) {
      const { errors: templateErrors, sanitized } = validateTemplateSubmission(
        template,
        fieldValues
      )
      if (Object.keys(templateErrors).length > 0) {
        setFieldErrors(templateErrors)
        return
      }
      sanitizedFields = sanitized
    }

    try {
      const metadata: Record<string, unknown> = {}
      const iconValue = values.icon?.trim()
      const colorValue = values.color?.trim()
      if (iconValue) {
        metadata.icon = iconValue
      }
      if (colorValue) {
        metadata.color = colorValue
      }

      const payload: ConnectionCreatePayload = {
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        protocol_id: protocol.id,
        folder_id: sanitizeId(values.folder_id),
        team_id: denormalizeTeamValue(values.team_id),
        metadata: Object.keys(metadata).length ? metadata : undefined,
        identity_id: selectedIdentityId,
      }

      if (sanitizedFields && Object.keys(sanitizedFields).length > 0) {
        payload.fields = sanitizedFields
      }

      if (effectiveTeamId && autoGrantTeamPermissions && missingTeamPermissionIds.length > 0) {
        const uniquePermissions = Array.from(new Set(missingTeamPermissionIds))
        if (uniquePermissions.length > 0) {
          payload.grant_team_permissions = uniquePermissions
        }
      }

      let savedConnection: ConnectionRecord
      if (mode === 'edit' && connection) {
        const updatePayload = {
          name: payload.name,
          description: payload.description,
          folder_id: payload.folder_id,
          team_id: payload.team_id,
          metadata: payload.metadata,
          fields: payload.fields,
          identity_id: payload.identity_id,
        }
        savedConnection = await update.mutateAsync({ id: connection.id, payload: updatePayload })
      } else {
        savedConnection = await create.mutateAsync(payload)
      }

      onSuccess(savedConnection)
      onClose()
    } catch (error) {
      const apiError = toApiError(error)
      setFormError(apiError)
    }
  }

  const isLoading =
    isSubmitting || create.isPending || update.isPending || (templateLoading && open && !!template)
  const folderOptions = useMemo(
    () => flattenFolders(folders, effectiveTeamId ?? null),
    [effectiveTeamId, folders]
  )

  useEffect(() => {
    if (!selectedFolderId) {
      return
    }
    const hasMatch = folderOptions.some((option) => option.id === selectedFolderId)
    if (!hasMatch) {
      setValue('folder_id', '', { shouldDirty: true })
    }
  }, [folderOptions, selectedFolderId, setValue])

  if (!protocol) {
    return null
  }

  const Icon = resolveProtocolIcon(protocol)
  const modalTitle =
    mode === 'edit' && connection ? `Edit ${connection.name}` : `Configure ${protocol.name}`
  const submitLabel = mode === 'edit' ? 'Save changes' : 'Create Connection'

  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        title={modalTitle}
        description={
          mode === 'edit'
            ? 'Update the connection details, access, and configuration.'
            : 'Provide connection metadata and protocol-specific configuration.'
        }
        size="lg"
      >
        <div className="flex flex-col gap-6">
          <div className="flex items-center gap-3 rounded-lg border border-border/70 bg-muted/20 p-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Icon className="h-5 w-5" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">{protocol.name}</p>
              <p className="text-xs text-muted-foreground">
                {protocol.description ?? 'No description provided.'}
              </p>
            </div>
          </div>

          <form className="space-y-5" onSubmit={handleSubmit(onSubmit)} autoComplete="off">
            <Input
              label="Connection name"
              placeholder="Production SSH"
              {...register('name')}
              error={errors.name?.message}
              disabled={isLoading}
            />

            <Textarea
              label="Description"
              placeholder="Optional - share context for teammates."
              rows={3}
              {...register('description')}
              error={errors.description?.message}
              disabled={isLoading}
            />

            <div className="grid gap-3">
              <label className="text-sm font-medium text-foreground">Icon</label>
              <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
                {(iconOptions.length ? iconOptions : CONNECTION_ICON_OPTIONS).map(
                  ({ id, label, icon: OptionIcon }) => {
                    const isActive = (selectedIcon || DEFAULT_CONNECTION_ICON_ID) === id
                    return (
                      <button
                        key={id}
                        type="button"
                        onClick={() => setValue('icon', id, { shouldDirty: true })}
                        className={cn(
                          'flex h-12 items-center justify-center gap-2 rounded-lg border text-sm transition-colors',
                          isActive
                            ? 'border-primary bg-primary/10 text-primary'
                            : 'border-border text-muted-foreground hover:border-border/80 hover:bg-muted/40'
                        )}
                        aria-pressed={isActive}
                        disabled={isLoading}
                      >
                        <OptionIcon className="h-4 w-4" />
                        <span className="truncate">{label}</span>
                      </button>
                    )
                  }
                )}
              </div>
            </div>

            <div className="grid gap-3">
              <label className="text-sm font-medium text-foreground">Accent color</label>
              <div className="flex flex-wrap gap-2">
                <ColorSwatch
                  key="none"
                  label="Default"
                  color=""
                  isActive={!selectedColor}
                  onSelect={() => setValue('color', '', { shouldDirty: true })}
                  disabled={isLoading}
                />
                {CONNECTION_COLOR_OPTIONS.map((option) => (
                  <ColorSwatch
                    key={option.id}
                    label={option.label}
                    color={option.value}
                    isActive={selectedColor === option.value}
                    onSelect={() => setValue('color', option.value, { shouldDirty: true })}
                    disabled={isLoading}
                  />
                ))}
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium text-foreground" htmlFor="connection-folder">
                Folder
              </label>
              <Controller
                name="folder_id"
                control={control}
                render={({ field }) => (
                  <Select
                    value={field.value ?? ''}
                    onValueChange={field.onChange}
                    disabled={isLoading}
                  >
                    <SelectTrigger id="connection-folder" className="h-10 w-full justify-between">
                      <SelectValue placeholder="Unassigned" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="">Unassigned</SelectItem>
                      {folderOptions.length === 0 ? (
                        <SelectItem value="__no_folders__" disabled>
                          Create a folder to organize connections
                        </SelectItem>
                      ) : null}
                      {folderOptions.map((option) => (
                        <SelectItem key={option.id} value={option.id}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              <p className="text-xs text-muted-foreground">
                Optional. Connections without a folder appear in the Unassigned view.
              </p>
            </div>

            {allowTeamAssignment && teams.length ? (
              <div className="flex flex-col gap-2">
                <label className="text-sm font-medium text-foreground" htmlFor="connection-team">
                  Team
                </label>
                <Controller
                  name="team_id"
                  control={control}
                  render={({ field }) => (
                    <Select
                      value={field.value && field.value.length ? field.value : ''}
                      onValueChange={field.onChange}
                      disabled={isLoading}
                    >
                      <SelectTrigger id="connection-team" className="h-10 w-full justify-between">
                        <SelectValue placeholder="Personal workspace" />
                      </SelectTrigger>
                      <SelectContent align="start">
                        <SelectItem value="">Personal workspace</SelectItem>
                        {teams.length === 0 ? (
                          <SelectItem value="__no_teams__" disabled>
                            Create a team to share this connection
                          </SelectItem>
                        ) : null}
                        {teams.map((team) => (
                          <SelectItem key={team.id} value={team.id}>
                            {team.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
                {effectiveTeamId ? (
                  <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                    {teamCapabilitiesQuery.isLoading ? (
                      'Checking team capabilities…'
                    ) : teamCapabilityWarnings.length === 0 ? (
                      'Team currently has the required permissions to launch this connection.'
                    ) : (
                      <div className="space-y-2">
                        <ul className="list-disc space-y-1 pl-4">
                          {teamCapabilityWarnings.map((warning) => (
                            <li key={warning}>{warning}</li>
                          ))}
                        </ul>
                        {missingTeamPermissionIds.length > 0 ? (
                          <label
                            htmlFor="auto-grant-team-permissions"
                            className="flex items-start gap-3 rounded-md border border-dashed border-border/60 bg-background px-3 py-2 text-foreground"
                          >
                            <Checkbox
                              id="auto-grant-team-permissions"
                              checked={autoGrantTeamPermissions}
                              onCheckedChange={(value) => handleAutoGrantToggle(value === true)}
                              className="mt-1"
                              disabled={isLoading}
                            />
                            <div className="space-y-1 text-xs text-muted-foreground">
                              <p className="font-medium text-foreground">
                                Grant missing permissions to{' '}
                                {selectedTeam?.name ?? 'the selected team'} for this connection.
                              </p>
                              <p>The team will receive:</p>
                              <div className="mt-1 flex flex-wrap gap-1.5">
                                {missingTeamPermissionIds.map((permissionId) => (
                                  <Badge
                                    key={permissionId}
                                    variant="secondary"
                                    className="text-[10px] uppercase tracking-wide"
                                  >
                                    {permissionId}
                                  </Badge>
                                ))}
                              </div>
                            </div>
                          </label>
                        ) : null}
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            ) : null}

            <div className="space-y-2">
              <label className="text-sm font-medium text-foreground">Credential identity</label>
              <IdentitySelector
                value={selectedIdentityId}
                onChange={setSelectedIdentityId}
                protocolId={protocol.id}
                allowInlineCreate={canCreateIdentity}
                onCreateIdentity={() => setIdentityModalOpen(true)}
                disabled={isLoading}
              />
              <p className="text-xs text-muted-foreground">
                Connections reference vault identities to securely access remote resources.
              </p>
            </div>

            {template ? (
              <div className="space-y-3">
                <div>
                  <p className="text-sm font-semibold text-foreground">Protocol configuration</p>
                  <p className="text-xs text-muted-foreground">
                    Provide the fields required by the {template.displayName.toLowerCase()} schema.
                  </p>
                </div>
                <ConnectionTemplateForm
                  template={template}
                  values={fieldValues}
                  errors={fieldErrors}
                  disabled={isLoading}
                  onChange={handleFieldChange}
                />
              </div>
            ) : templateLoading ? (
              <div className="rounded-lg border border-border/60 bg-muted/10 px-4 py-5 text-xs text-muted-foreground">
                Loading protocol template…
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/60 bg-muted/10 px-4 py-5 text-xs text-muted-foreground">
                This protocol does not declare additional configuration fields.
              </div>
            )}

            {formError ? (
              <div className="rounded-md border border-rose-500/60 bg-rose-500/10 px-3 py-2 text-sm text-rose-500">
                {formError.message}
              </div>
            ) : null}

            <div className="flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={onClose} disabled={isLoading}>
                Cancel
              </Button>
              <Button type="submit" disabled={isLoading}>
                {submitLabel}
              </Button>
            </div>
          </form>
        </div>
      </Modal>

      <IdentityFormModal
        open={identityModalOpen}
        onClose={() => setIdentityModalOpen(false)}
        mode="create"
        onSuccess={(identity) => {
          setSelectedIdentityId(identity.id)
          setIdentityModalOpen(false)
        }}
      />
    </>
  )
}

type FlattenedFolder = {
  id: string
  label: string
}

function matchesTeam(folderTeamId: string | null | undefined, teamId: string | null): boolean {
  const normalizedFolderTeam = folderTeamId ?? null
  if (teamId === null) {
    return normalizedFolderTeam === null
  }
  return normalizedFolderTeam === teamId
}

function flattenFolders(
  nodes: ConnectionFolderNode[],
  teamId: string | null,
  prefix: string[] = []
): FlattenedFolder[] {
  const result: FlattenedFolder[] = []
  nodes.forEach((node) => {
    const folderTeamId = node.folder.team_id ?? null
    const matches = matchesTeam(folderTeamId, teamId)
    const nextPrefix = matches ? [...prefix, node.folder.name] : prefix

    if (matches) {
      result.push({
        id: node.folder.id,
        label: nextPrefix.join(' / '),
      })
    }

    if (node.children?.length) {
      result.push(...flattenFolders(node.children, teamId, nextPrefix))
    }
  })
  return result
}

function sanitizeId(value?: string | null | undefined): string | null | undefined {
  if (typeof value !== 'string') {
    return value
  }
  const trimmed = value.trim()
  if (!trimmed || trimmed === '__no_folders__') {
    return null
  }
  return trimmed
}

function normalizeTeamValue(value?: string | null): string {
  if (!value) {
    return ''
  }
  return value
}

function denormalizeTeamValue(value?: string | null): string | null {
  if (!value) {
    return null
  }
  if (value === '__no_teams__') {
    return null
  }
  return value
}

function extractTeamCapabilityWarnings(capabilities: { permission_ids?: string[] }): {
  missingPermissionIds: string[]
  messages: string[]
} {
  const granted = new Set(
    (capabilities.permission_ids ?? []).map((permission) => permission.trim())
  )
  const required = ['connection.launch']
  const missing: string[] = []
  const messages: string[] = []

  required.forEach((permissionId) => {
    if (!granted.has(permissionId)) {
      missing.push(permissionId)
      messages.push(`Team is missing the ${permissionId} permission.`)
    }
  })

  return {
    missingPermissionIds: missing,
    messages,
  }
}

type TemplateValidationResult = {
  errors: Record<string, string>
  sanitized: Record<string, unknown>
}

function validateTemplateSubmission(
  template: ConnectionTemplate,
  rawValues: TemplateValueMap
): TemplateValidationResult {
  const errors: Record<string, string> = {}
  const sanitized: Record<string, unknown> = {}
  const workingValues: TemplateValueMap = { ...rawValues }

  template.sections.forEach((section) => {
    section.fields.forEach((field) => {
      const visible = isFieldVisible(field, workingValues)
      if (!visible) {
        return
      }

      const raw = rawValues[field.key]
      const { value, error } = coerceFieldValueForSubmit(field.type, raw, field)
      if (error) {
        errors[field.key] = error
        workingValues[field.key] = raw
        return
      }

      workingValues[field.key] = value

      if (value === undefined) {
        if (field.required) {
          errors[field.key] = `${field.label} is required`
        }
        return
      }

      if (!validateAgainstFieldRules(field, value)) {
        errors[field.key] = buildValidationMessage(field)
        return
      }

      sanitized[field.key] = value
    })
  })

  return { errors, sanitized }
}

function coerceFieldValueForSubmit(
  type: string,
  raw: unknown,
  field: ConnectionTemplate['sections'][number]['fields'][number]
): { value: unknown; error?: string } {
  if (raw === null || raw === undefined || raw === '') {
    if (field.default !== undefined) {
      return { value: field.default }
    }
    return { value: undefined }
  }

  switch (type) {
    case 'boolean':
      return { value: Boolean(raw) }
    case 'number':
    case 'target_port': {
      const numeric = typeof raw === 'number' ? raw : Number(raw)
      if (!Number.isFinite(numeric)) {
        return { value: undefined, error: `${field.label} must be a number` }
      }
      return { value: numeric }
    }
    case 'json': {
      if (typeof raw === 'object') {
        return { value: raw }
      }
      if (typeof raw === 'string') {
        if (!raw.trim()) {
          return { value: undefined }
        }
        try {
          const parsed = JSON.parse(raw)
          return { value: parsed }
        } catch {
          return { value: undefined, error: `${field.label} must be valid JSON` }
        }
      }
      return { value: undefined, error: `${field.label} must be valid JSON` }
    }
    default: {
      const text = typeof raw === 'string' ? raw.trim() : String(raw)
      if (!text) {
        return { value: undefined }
      }
      return { value: text }
    }
  }
}

function validateAgainstFieldRules(
  field: ConnectionTemplate['sections'][number]['fields'][number],
  value: unknown
): boolean {
  const validation = field.validation ?? {}
  if (value === undefined) {
    return !field.required
  }

  if (typeof value === 'string') {
    if (typeof validation.pattern === 'string' && validation.pattern.length > 0) {
      try {
        const regex = new RegExp(validation.pattern)
        if (!regex.test(value)) {
          return false
        }
      } catch {
        // ignore invalid patterns
      }
    }
    if (isFiniteNumber(validation.min_length) && value.length < Number(validation.min_length)) {
      return false
    }
    if (isFiniteNumber(validation.max_length) && value.length > Number(validation.max_length)) {
      return false
    }
  }

  if (typeof value === 'number') {
    if (isFiniteNumber(validation.min) && value < Number(validation.min)) {
      return false
    }
    if (isFiniteNumber(validation.max) && value > Number(validation.max)) {
      return false
    }
  }

  if (field.type === 'select' && field.options?.length) {
    const allowed = field.options.map((option) => option.value)
    if (!allowed.includes(String(value))) {
      return false
    }
  }

  return true
}

function buildValidationMessage(
  field: ConnectionTemplate['sections'][number]['fields'][number]
): string {
  if (field.validation?.pattern) {
    return `${field.label} has an invalid format`
  }
  if (isFiniteNumber(field.validation?.min) || isFiniteNumber(field.validation?.max)) {
    return `${field.label} is out of range`
  }
  return `${field.label} is invalid`
}

function isFiniteNumber(value: unknown): boolean {
  if (typeof value === 'number') {
    return Number.isFinite(value)
  }
  if (typeof value === 'string' && value.trim().length > 0) {
    return Number.isFinite(Number(value))
  }
  return false
}

function buildInitialFieldValues(
  template: ConnectionTemplate,
  existingFields: Record<string, unknown>
): TemplateValueMap {
  const result: TemplateValueMap = {}
  template.sections.forEach((section) => {
    section.fields.forEach((field) => {
      const existing = existingFields[field.key]
      if (existing !== undefined) {
        result[field.key] = existing
      } else if (field.default !== undefined) {
        result[field.key] = field.default
      } else if (field.type === 'boolean') {
        result[field.key] = false
      } else {
        result[field.key] = undefined
      }
    })
  })

  return result
}

function extractTemplateMetadata(
  metadata?: ConnectionRecord['metadata']
): ConnectionTemplateMetadata | undefined {
  if (!metadata || typeof metadata !== 'object') {
    return undefined
  }
  const templateMetadata = metadata.connection_template
  if (templateMetadata && typeof templateMetadata === 'object') {
    return templateMetadata as ConnectionTemplateMetadata
  }
  return undefined
}

interface ColorSwatchProps {
  label: string
  color: string
  isActive: boolean
  onSelect: () => void
  disabled?: boolean
}

function ColorSwatch({ label, color, isActive, onSelect, disabled }: ColorSwatchProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'flex h-9 items-center gap-2 rounded-md border px-3 text-sm transition-colors',
        isActive
          ? 'border-primary bg-primary/10 text-primary'
          : 'border-border text-muted-foreground hover:border-border/80 hover:bg-muted/40'
      )}
      aria-pressed={isActive}
      disabled={disabled}
    >
      <span
        className="inline-block h-4 w-4 rounded-full border border-border/70"
        style={{ backgroundColor: color || 'transparent' }}
      />
      <span className="truncate">{label}</span>
    </button>
  )
}
