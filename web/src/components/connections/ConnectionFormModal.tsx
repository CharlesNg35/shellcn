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
import type { Protocol } from '@/types/protocols'
import type { ConnectionFolderNode, ConnectionRecord } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { resolveProtocolIcon } from '@/lib/utils/protocolIcons'
import { useConnectionMutations } from '@/hooks/useConnectionMutations'
import { ApiError, toApiError } from '@/lib/api/http'
import { teamsApi } from '@/lib/api/teams'
import type { ConnectionCreatePayload } from '@/lib/api/connections'
import { fetchSSHProtocolSettings } from '@/lib/api/protocol-settings'
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

const sessionOverrideSchema = z.object({
  use_custom: z.boolean(),
  concurrent_limit: z
    .number()
    .min(0, 'Concurrent limit must be zero or greater')
    .max(1000, 'Concurrent limit must be 1000 or less'),
  idle_timeout_minutes: z
    .number()
    .min(0, 'Idle timeout must be zero or greater')
    .max(10080, 'Idle timeout must be less than 10081 minutes'),
  enable_sftp: z.boolean(),
})

const terminalOverrideSchema = z.object({
  use_custom: z.boolean(),
  font_family: z
    .string()
    .trim()
    .min(1, 'Font family is required')
    .max(128, 'Font family must be at most 128 characters'),
  font_size: z
    .number()
    .min(8, 'Font size must be at least 8')
    .max(96, 'Font size must be at most 96'),
  scrollback_limit: z
    .number()
    .min(200, 'Scrollback must be at least 200 lines')
    .max(10000, 'Scrollback must be at most 10000 lines'),
  enable_webgl: z.boolean(),
})

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
  overrides: z.object({
    session: sessionOverrideSchema,
    terminal: terminalOverrideSchema,
  }),
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
  const [formError, setFormError] = useState<ApiError | null>(null)
  const grantToggleInteractedRef = useRef(false)
  const recordingDefaultAppliedRef = useRef(false)
  const [autoGrantTeamPermissions, setAutoGrantTeamPermissions] = useState(false)
  const { hasPermission } = usePermissions()
  const canCreateIdentity = hasPermission(PERMISSIONS.VAULT.CREATE)
  const [selectedIdentityId, setSelectedIdentityId] = useState<string | null>(null)
  const [identityModalOpen, setIdentityModalOpen] = useState(false)
  const [recordingOptIn, setRecordingOptIn] = useState(false)
  const formInitializedRef = useRef(false)

  const iconOptions = useMemo(() => {
    return getIconOptionsForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const defaultIcon = useMemo(() => {
    return getDefaultIconForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const { data: sshSettings, isLoading: sshSettingsLoading } = useQuery({
    queryKey: ['protocol-settings', 'ssh'],
    queryFn: fetchSSHProtocolSettings,
    enabled: open && protocol?.id === 'ssh',
    staleTime: 60_000,
  })

  const sessionDefaults = useMemo(() => {
    return {
      concurrent_limit: sshSettings?.session.concurrent_limit ?? 0,
      idle_timeout_minutes: sshSettings?.session.idle_timeout_minutes ?? 0,
      enable_sftp: sshSettings?.session.enable_sftp ?? true,
    }
  }, [
    sshSettings?.session.concurrent_limit,
    sshSettings?.session.enable_sftp,
    sshSettings?.session.idle_timeout_minutes,
  ])

  const terminalDefaults = useMemo(() => {
    return {
      font_family: sshSettings?.terminal.font_family ?? 'monospace',
      font_size: sshSettings?.terminal.font_size ?? 14,
      scrollback_limit: sshSettings?.terminal.scrollback_limit ?? 1000,
      enable_webgl: sshSettings?.terminal.enable_webgl ?? true,
    }
  }, [
    sshSettings?.terminal.enable_webgl,
    sshSettings?.terminal.font_family,
    sshSettings?.terminal.font_size,
    sshSettings?.terminal.scrollback_limit,
  ])

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
      overrides: {
        session: {
          use_custom: false,
          concurrent_limit: 0,
          idle_timeout_minutes: 0,
          enable_sftp: true,
        },
        terminal: {
          use_custom: false,
          font_family: 'monospace',
          font_size: 14,
          scrollback_limit: 1000,
          enable_webgl: true,
        },
      },
    },
  })

  useEffect(() => {
    if (!open) {
      setRecordingOptIn(false)
      recordingDefaultAppliedRef.current = false
      formInitializedRef.current = false
      return
    }
    if (protocol?.id === 'ssh' && sshSettingsLoading && !formInitializedRef.current) {
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

    const settings = connection?.settings ?? {}
    const sessionOverrideEnabled =
      typeof settings.concurrent_limit === 'number' ||
      typeof settings.idle_timeout_minutes === 'number' ||
      typeof settings.enable_sftp === 'boolean'

    const terminalOverrides = settings.terminal_config_override ?? undefined

    const initialValues: ConnectionFormValues = {
      name: connection?.name ?? '',
      description: connection?.description ?? '',
      folder_id: connection?.folder_id ?? '',
      team_id: normalizeTeamValue(connection?.team_id ?? teamId),
      icon: iconFromMetadata ?? defaultIcon ?? DEFAULT_CONNECTION_ICON_ID,
      color: colorFromMetadata,
      overrides: {
        session: {
          use_custom: sessionOverrideEnabled,
          concurrent_limit: sessionOverrideEnabled
            ? typeof settings.concurrent_limit === 'number'
              ? settings.concurrent_limit
              : sessionDefaults.concurrent_limit
            : sessionDefaults.concurrent_limit,
          idle_timeout_minutes: sessionOverrideEnabled
            ? typeof settings.idle_timeout_minutes === 'number'
              ? settings.idle_timeout_minutes
              : sessionDefaults.idle_timeout_minutes
            : sessionDefaults.idle_timeout_minutes,
          enable_sftp:
            sessionOverrideEnabled && typeof settings.enable_sftp === 'boolean'
              ? Boolean(settings.enable_sftp)
              : sessionDefaults.enable_sftp,
        },
        terminal: {
          use_custom: Boolean(terminalOverrides),
          font_family:
            terminalOverrides && terminalOverrides.font_family
              ? String(terminalOverrides.font_family)
              : terminalDefaults.font_family,
          font_size:
            terminalOverrides && typeof terminalOverrides.font_size === 'number'
              ? Number(terminalOverrides.font_size)
              : terminalDefaults.font_size,
          scrollback_limit:
            terminalOverrides && typeof terminalOverrides.scrollback_limit === 'number'
              ? Number(terminalOverrides.scrollback_limit)
              : terminalDefaults.scrollback_limit,
          enable_webgl:
            terminalOverrides && terminalOverrides.enable_webgl !== undefined
              ? Boolean(terminalOverrides.enable_webgl)
              : terminalDefaults.enable_webgl,
        },
      },
    }

    reset(initialValues)
    setValue('overrides.session.use_custom', initialValues.overrides.session.use_custom, {
      shouldDirty: false,
    })
    setValue('overrides.terminal.use_custom', initialValues.overrides.terminal.use_custom, {
      shouldDirty: false,
    })
    setFormError(null)
    grantToggleInteractedRef.current = false
    setAutoGrantTeamPermissions(false)
    setSelectedIdentityId(connection?.identity_id ?? null)

    if (mode === 'edit' && typeof settings.recording_enabled === 'boolean') {
      setRecordingOptIn(Boolean(settings.recording_enabled))
      recordingDefaultAppliedRef.current = true
    } else {
      setRecordingOptIn(false)
      recordingDefaultAppliedRef.current = false
    }

    formInitializedRef.current = true
  }, [
    connection,
    defaultIcon,
    mode,
    open,
    protocol?.id,
    reset,
    sessionDefaults,
    setValue,
    sshSettingsLoading,
    teamId,
    terminalDefaults,
  ])

  const selectedIcon = watch('icon')
  const selectedColor = watch('color')
  const selectedTeamValue = watch('team_id')
  const sessionOverride = watch('overrides.session')
  const terminalOverride = watch('overrides.terminal')

  const usingSessionDefaults = !sessionOverride?.use_custom
  const usingTerminalDefaults = !terminalOverride?.use_custom

  const handleSessionOverrideToggle = (checked: boolean) => {
    setValue('overrides.session.use_custom', checked, { shouldDirty: true })
    if (!checked) {
      setValue('overrides.session.concurrent_limit', sessionDefaults.concurrent_limit, {
        shouldDirty: true,
      })
      setValue('overrides.session.idle_timeout_minutes', sessionDefaults.idle_timeout_minutes, {
        shouldDirty: true,
      })
      setValue('overrides.session.enable_sftp', sessionDefaults.enable_sftp, {
        shouldDirty: true,
      })
    }
  }

  const handleTerminalOverrideToggle = (checked: boolean) => {
    setValue('overrides.terminal.use_custom', checked, { shouldDirty: true })
    if (!checked) {
      setValue('overrides.terminal.font_family', terminalDefaults.font_family, {
        shouldDirty: true,
      })
      setValue('overrides.terminal.font_size', terminalDefaults.font_size, {
        shouldDirty: true,
      })
      setValue('overrides.terminal.scrollback_limit', terminalDefaults.scrollback_limit, {
        shouldDirty: true,
      })
      setValue('overrides.terminal.enable_webgl', terminalDefaults.enable_webgl, {
        shouldDirty: true,
      })
    }
  }

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

  const teamCapabilityAnalysis = useMemo(() => {
    const result = {
      missingPermissionIds: [] as string[],
      messages: [] as string[],
    }
    if (!effectiveTeamId) {
      return result
    }
    const capabilities = teamCapabilitiesQuery.data
    if (!capabilities) {
      return result
    }
    const granted = new Set(capabilities.permission_ids ?? [])

    if (!granted.has('connection.launch')) {
      result.missingPermissionIds.push('connection.launch')
      result.messages.push(
        'Team members will not be able to launch this connection without granting connection.launch.'
      )
    }
    if (!granted.has('connection.manage')) {
      result.messages.push(
        'Team members will not be able to edit this connection (missing connection.manage).'
      )
    }
    if (protocol) {
      const connectPermissionId = `protocol:${protocol.id}.connect`
      if (!granted.has(connectPermissionId)) {
        result.missingPermissionIds.push(connectPermissionId)
        result.messages.push(
          `Team currently lacks ${protocol.name} protocol access (missing ${connectPermissionId}).`
        )
      }
    }
    return result
  }, [effectiveTeamId, teamCapabilitiesQuery.data, protocol])

  const missingTeamPermissionIds = teamCapabilityAnalysis.missingPermissionIds
  const teamCapabilityWarnings = teamCapabilityAnalysis.messages

  const recordingMode =
    protocol?.id === 'ssh' ? (sshSettings?.recording.mode ?? 'optional') : undefined
  const recordingRetentionDays =
    protocol?.id === 'ssh' ? (sshSettings?.recording.retention_days ?? 0) : 0

  const handleAutoGrantToggle = (checked: boolean) => {
    grantToggleInteractedRef.current = true
    setAutoGrantTeamPermissions(checked)
  }

  useEffect(() => {
    if (!open) {
      return
    }
    if (missingTeamPermissionIds.length === 0) {
      return
    }
    if (!grantToggleInteractedRef.current) {
      setAutoGrantTeamPermissions(true)
    }
  }, [missingTeamPermissionIds.length, open])

  useEffect(() => {
    if (missingTeamPermissionIds.length === 0) {
      grantToggleInteractedRef.current = false
      setAutoGrantTeamPermissions(false)
    }
  }, [missingTeamPermissionIds.length])

  useEffect(() => {
    if (!open) {
      return
    }
    if (recordingDefaultAppliedRef.current) {
      return
    }
    if (protocol?.id !== 'ssh') {
      recordingDefaultAppliedRef.current = true
      setRecordingOptIn(false)
      return
    }
    const mode = sshSettings?.recording.mode
    if (!mode) {
      if (!sshSettingsLoading) {
        recordingDefaultAppliedRef.current = true
        setRecordingOptIn(false)
      }
      return
    }
    if (mode === 'forced') {
      setRecordingOptIn(true)
    } else {
      setRecordingOptIn(false)
    }
    recordingDefaultAppliedRef.current = true
  }, [open, protocol?.id, sshSettings?.recording.mode, sshSettingsLoading])

  useEffect(() => {
    if (!selectedIcon) {
      setValue('icon', defaultIcon ?? DEFAULT_CONNECTION_ICON_ID, { shouldValidate: false })
    }
  }, [defaultIcon, selectedIcon, setValue])

  const onSubmit: SubmitHandler<ConnectionFormValues> = async (values) => {
    setFormError(null)
    if (!protocol) {
      return
    }
    if (!selectedIdentityId) {
      setFormError(
        new ApiError({
          code: 'validation.identity_required',
          message: 'Select or create a vault identity to continue.',
        })
      )
      return
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

      const settings: Record<string, unknown> = {}
      if (protocol.id === 'ssh') {
        if (recordingMode === 'forced') {
          settings.recording_enabled = true
        } else if (recordingMode === 'optional') {
          settings.recording_enabled = recordingOptIn
        }
        if (values.overrides.session.use_custom) {
          settings.concurrent_limit = values.overrides.session.concurrent_limit
          settings.idle_timeout_minutes = values.overrides.session.idle_timeout_minutes
          settings.enable_sftp = values.overrides.session.enable_sftp
        }
        if (values.overrides.terminal.use_custom) {
          settings.terminal_config_override = {
            font_family: values.overrides.terminal.font_family.trim(),
            font_size: values.overrides.terminal.font_size,
            scrollback_limit: values.overrides.terminal.scrollback_limit,
            enable_webgl: values.overrides.terminal.enable_webgl,
          }
        }
      }
      if (Object.keys(settings).length > 0) {
        payload.settings = settings
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
          settings: payload.settings,
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

  const isLoading = isSubmitting || create.isPending || update.isPending
  const folderOptions = useMemo(() => flattenFolders(folders), [folders])

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
            ? 'Update the connection details and overrides for this resource.'
            : 'Provide a name and optional folder to keep things organized.'
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
            />

            <Textarea
              label="Description"
              placeholder="Optional - share context for teammates."
              rows={3}
              {...register('description')}
              error={errors.description?.message}
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
                        onClick={() => setValue('icon', id, { shouldValidate: true })}
                        className={cn(
                          'flex h-12 items-center justify-center gap-2 rounded-lg border text-sm transition-colors',
                          isActive
                            ? 'border-primary bg-primary/10 text-primary'
                            : 'border-border text-muted-foreground hover:border-border/80 hover:bg-muted/40'
                        )}
                        aria-pressed={isActive}
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
                  onSelect={() => setValue('color', '', { shouldValidate: true })}
                />
                {CONNECTION_COLOR_OPTIONS.map((option) => (
                  <ColorSwatch
                    key={option.id}
                    label={option.label}
                    color={option.value}
                    isActive={selectedColor === option.value}
                    onSelect={() => setValue('color', option.value, { shouldValidate: true })}
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
                  <Select value={field.value ?? ''} onValueChange={field.onChange}>
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
              />
              <p className="text-xs text-muted-foreground">
                Connections reference vault identities to securely access remote resources. Select
                an existing credential or create a connection-scoped identity.
              </p>
            </div>

            {protocol.id === 'ssh' ? (
              <div className="space-y-4 rounded-lg border border-border/60 bg-muted/10 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-foreground">Session behaviour</p>
                    <p className="text-xs text-muted-foreground">
                      Control concurrency, idle timeout, and SFTP access for this connection.
                    </p>
                  </div>
                  <label
                    htmlFor="session-override-toggle"
                    className="flex cursor-pointer items-center gap-2 text-sm text-foreground"
                  >
                    <Checkbox
                      id="session-override-toggle"
                      checked={!usingSessionDefaults}
                      onCheckedChange={(checked) => handleSessionOverrideToggle(Boolean(checked))}
                      disabled={isLoading || sshSettingsLoading}
                    />
                    <span>Customise session values</span>
                  </label>
                </div>

                <div className="grid gap-4 md:grid-cols-3">
                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium text-foreground"
                      htmlFor="override-concurrent-limit"
                    >
                      Concurrent sessions
                    </label>
                    <Input
                      id="override-concurrent-limit"
                      type="number"
                      min={0}
                      max={1000}
                      value={sessionOverride?.concurrent_limit ?? sessionDefaults.concurrent_limit}
                      onChange={(event) =>
                        setValue(
                          'overrides.session.concurrent_limit',
                          Number(event.target.value) || 0,
                          { shouldDirty: true }
                        )
                      }
                      disabled={usingSessionDefaults || isLoading}
                    />
                    <p className="text-xs text-muted-foreground">
                      Default: {sessionDefaults.concurrent_limit || 'Unlimited'} concurrent session
                      {sessionDefaults.concurrent_limit === 1 ? '' : 's'}.
                    </p>
                    {errors.overrides?.session?.concurrent_limit ? (
                      <p className="text-xs text-rose-500">
                        {errors.overrides.session.concurrent_limit.message}
                      </p>
                    ) : null}
                  </div>

                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium text-foreground"
                      htmlFor="override-idle-timeout"
                    >
                      Idle timeout (minutes)
                    </label>
                    <Input
                      id="override-idle-timeout"
                      type="number"
                      min={0}
                      max={10080}
                      value={
                        sessionOverride?.idle_timeout_minutes ??
                        sessionDefaults.idle_timeout_minutes
                      }
                      onChange={(event) =>
                        setValue(
                          'overrides.session.idle_timeout_minutes',
                          Number(event.target.value) || 0,
                          { shouldDirty: true }
                        )
                      }
                      disabled={usingSessionDefaults || isLoading}
                    />
                    <p className="text-xs text-muted-foreground">
                      Default: {sessionDefaults.idle_timeout_minutes} minute
                      {sessionDefaults.idle_timeout_minutes === 1 ? '' : 's'}.
                    </p>
                    {errors.overrides?.session?.idle_timeout_minutes ? (
                      <p className="text-xs text-rose-500">
                        {errors.overrides.session.idle_timeout_minutes.message}
                      </p>
                    ) : null}
                  </div>

                  <div className="space-y-2">
                    <span className="text-sm font-medium text-foreground">SFTP access</span>
                    <label
                      htmlFor="override-sftp"
                      className="flex cursor-pointer items-start gap-3 rounded-md border border-border/60 bg-background px-3 py-2"
                    >
                      <Checkbox
                        id="override-sftp"
                        checked={sessionOverride?.enable_sftp ?? sessionDefaults.enable_sftp}
                        onCheckedChange={(checked) =>
                          setValue('overrides.session.enable_sftp', Boolean(checked), {
                            shouldDirty: true,
                          })
                        }
                        disabled={usingSessionDefaults || isLoading}
                      />
                      <div className="space-y-1">
                        <p className="text-xs font-medium text-foreground">Allow SFTP</p>
                        <p className="text-xs text-muted-foreground">
                          Default is {sessionDefaults.enable_sftp ? 'enabled' : 'disabled'}.
                        </p>
                      </div>
                    </label>
                  </div>
                </div>
              </div>
            ) : null}

            {protocol.id === 'ssh' ? (
              <div className="space-y-4 rounded-lg border border-border/60 bg-muted/10 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-foreground">Terminal appearance</p>
                    <p className="text-xs text-muted-foreground">
                      Adjust font, size, and scrollback for terminals launched from this connection.
                    </p>
                  </div>
                  <label
                    htmlFor="terminal-override-toggle"
                    className="flex cursor-pointer items-center gap-2 text-sm text-foreground"
                  >
                    <Checkbox
                      id="terminal-override-toggle"
                      checked={!usingTerminalDefaults}
                      onCheckedChange={(checked) => handleTerminalOverrideToggle(Boolean(checked))}
                      disabled={isLoading || sshSettingsLoading}
                    />
                    <span>Customise terminal values</span>
                  </label>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium text-foreground"
                      htmlFor="override-font-family"
                    >
                      Font family
                    </label>
                    <Input
                      id="override-font-family"
                      value={terminalOverride?.font_family ?? terminalDefaults.font_family}
                      onChange={(event) =>
                        setValue('overrides.terminal.font_family', event.target.value, {
                          shouldDirty: true,
                        })
                      }
                      disabled={usingTerminalDefaults || isLoading}
                      placeholder="e.g. Fira Code"
                    />
                    <p className="text-xs text-muted-foreground">
                      Default: {terminalDefaults.font_family}.
                    </p>
                    {errors.overrides?.terminal?.font_family ? (
                      <p className="text-xs text-rose-500">
                        {errors.overrides.terminal.font_family.message}
                      </p>
                    ) : null}
                  </div>

                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium text-foreground"
                      htmlFor="override-font-size"
                    >
                      Font size (px)
                    </label>
                    <Input
                      id="override-font-size"
                      type="number"
                      min={8}
                      max={96}
                      value={terminalOverride?.font_size ?? terminalDefaults.font_size}
                      onChange={(event) =>
                        setValue(
                          'overrides.terminal.font_size',
                          Number(event.target.value) || terminalDefaults.font_size,
                          { shouldDirty: true }
                        )
                      }
                      disabled={usingTerminalDefaults || isLoading}
                    />
                    <p className="text-xs text-muted-foreground">
                      Default: {terminalDefaults.font_size}px.
                    </p>
                    {errors.overrides?.terminal?.font_size ? (
                      <p className="text-xs text-rose-500">
                        {errors.overrides.terminal.font_size.message}
                      </p>
                    ) : null}
                  </div>

                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium text-foreground"
                      htmlFor="override-scrollback"
                    >
                      Scrollback limit
                    </label>
                    <Input
                      id="override-scrollback"
                      type="number"
                      min={200}
                      max={10000}
                      value={
                        terminalOverride?.scrollback_limit ?? terminalDefaults.scrollback_limit
                      }
                      onChange={(event) =>
                        setValue(
                          'overrides.terminal.scrollback_limit',
                          Number(event.target.value) || terminalDefaults.scrollback_limit,
                          { shouldDirty: true }
                        )
                      }
                      disabled={usingTerminalDefaults || isLoading}
                    />
                    <p className="text-xs text-muted-foreground">
                      Default: {terminalDefaults.scrollback_limit} lines.
                    </p>
                    {errors.overrides?.terminal?.scrollback_limit ? (
                      <p className="text-xs text-rose-500">
                        {errors.overrides.terminal.scrollback_limit.message}
                      </p>
                    ) : null}
                  </div>

                  <div className="space-y-2">
                    <span className="text-sm font-medium text-foreground">WebGL acceleration</span>
                    <label
                      htmlFor="override-webgl"
                      className="flex cursor-pointer items-start gap-3 rounded-md border border-border/60 bg-background px-3 py-2"
                    >
                      <Checkbox
                        id="override-webgl"
                        checked={terminalOverride?.enable_webgl ?? terminalDefaults.enable_webgl}
                        onCheckedChange={(checked) =>
                          setValue('overrides.terminal.enable_webgl', Boolean(checked), {
                            shouldDirty: true,
                          })
                        }
                        disabled={usingTerminalDefaults || isLoading}
                      />
                      <div className="space-y-1">
                        <p className="text-xs font-medium text-foreground">Enable WebGL</p>
                        <p className="text-xs text-muted-foreground">
                          Default is {terminalDefaults.enable_webgl ? 'enabled' : 'disabled'}.
                        </p>
                      </div>
                    </label>
                  </div>
                </div>
              </div>
            ) : null}

            {protocol.id === 'ssh' ? (
              <div className="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-4">
                <div className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-1">
                      <p className="text-sm font-medium text-foreground">Session recording</p>
                      <p className="text-xs text-muted-foreground">
                        {recordingMode === 'forced'
                          ? 'Recording is enforced for all SSH sessions.'
                          : recordingMode === 'disabled'
                            ? 'Administrators have disabled session recording for SSH.'
                            : 'Enable capture so terminal activity from this connection can be reviewed later.'}
                      </p>
                    </div>
                    {recordingMode === 'optional' ? (
                      <label
                        htmlFor="recording-enabled"
                        className="flex cursor-pointer items-center gap-2 text-sm text-foreground"
                      >
                        <Checkbox
                          id="recording-enabled"
                          checked={recordingOptIn}
                          onCheckedChange={(checked) => setRecordingOptIn(Boolean(checked))}
                          disabled={sshSettingsLoading || isLoading}
                        />
                        <span>Record sessions launched from this connection</span>
                      </label>
                    ) : (
                      <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                        {recordingMode === 'forced' ? 'Forced' : 'Disabled'}
                      </Badge>
                    )}
                  </div>

                  <div className="text-xs text-muted-foreground">
                    {sshSettingsLoading
                      ? 'Loading recording defaults…'
                      : recordingMode === 'forced'
                        ? 'All sessions launched from this connection will be captured automatically.'
                        : recordingMode === 'disabled'
                          ? 'Recording cannot be enabled unless administrators change the global policy.'
                          : recordingOptIn
                            ? 'Recording starts automatically and the playback will appear under Settings → Protocols.'
                            : 'You can still enable recording after launching a session when policies allow it.'}
                  </div>
                  {recordingMode !== 'disabled' && recordingRetentionDays > 0 ? (
                    <p className="text-xs text-muted-foreground">
                      Recordings are retained for {recordingRetentionDays} day
                      {recordingRetentionDays === 1 ? '' : 's'}.
                    </p>
                  ) : null}
                </div>
              </div>
            ) : null}

            <div className="rounded-lg border border-dashed border-border/60 bg-muted/10 px-3 py-2">
              <p className="text-xs text-muted-foreground">
                Additional protocol-specific fields are coming soon. You can revisit this connection
                later to add advanced settings.
              </p>
            </div>

            {formError ? (
              <div className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {formError.message}
              </div>
            ) : null}

            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={onClose} disabled={isLoading}>
                Cancel
              </Button>
              <Button type="submit" loading={isLoading}>
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
        defaultScope="connection"
        connectionId={null}
        onSuccess={(identity) => {
          setSelectedIdentityId(identity.id)
          setIdentityModalOpen(false)
        }}
      />
    </>
  )
}

interface FolderOption {
  id: string
  label: string
}

interface ColorSwatchProps {
  label: string
  color: string
  isActive: boolean
  onSelect: () => void
}

function ColorSwatch({ label, color, isActive, onSelect }: ColorSwatchProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium transition-colors',
        isActive
          ? 'border-primary bg-primary/10 text-primary'
          : 'border-border text-muted-foreground hover:bg-muted/40'
      )}
      aria-pressed={isActive}
    >
      <span
        className="h-3 w-3 rounded-full border border-border/40"
        style={{ backgroundColor: color || 'transparent' }}
      />
      {label}
    </button>
  )
}

function flattenFolders(nodes: ConnectionFolderNode[], depth = 0): FolderOption[] {
  const options: FolderOption[] = []
  const indent = depth > 0 ? `${'  '.repeat(depth)}` : ''

  for (const node of nodes) {
    if (node.folder.id === 'unassigned') {
      if (node.children) {
        options.push(...flattenFolders(node.children, depth))
      }
      continue
    }
    options.push({
      id: node.folder.id,
      label: `${indent}${node.folder.name}`,
    })
    if (node.children?.length) {
      options.push(...flattenFolders(node.children, depth + 1))
    }
  }

  return options
}

function sanitizeId(value?: string | null) {
  const trimmed = value?.trim()
  return trimmed ? trimmed : null
}

function normalizeTeamValue(teamId?: string | null) {
  if (!teamId || teamId === 'personal') {
    return ''
  }
  return teamId
}

function denormalizeTeamValue(teamId?: string | null) {
  const trimmed = teamId?.trim()
  if (!trimmed || trimmed === 'personal') {
    return null
  }
  return trimmed
}
