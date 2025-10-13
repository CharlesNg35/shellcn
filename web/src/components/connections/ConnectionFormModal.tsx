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
}: ConnectionFormModalProps) {
  const { create } = useConnectionMutations()
  const [formError, setFormError] = useState<ApiError | null>(null)
  const grantToggleInteractedRef = useRef(false)
  const [autoGrantTeamPermissions, setAutoGrantTeamPermissions] = useState(false)
  const { hasPermission } = usePermissions()
  const canCreateIdentity = hasPermission(PERMISSIONS.VAULT.CREATE)
  const [selectedIdentityId, setSelectedIdentityId] = useState<string | null>(null)
  const [identityModalOpen, setIdentityModalOpen] = useState(false)

  const iconOptions = useMemo(() => {
    return getIconOptionsForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const defaultIcon = useMemo(() => {
    return getDefaultIconForProtocol(protocol?.id, protocol?.category)
  }, [protocol?.category, protocol?.id])

  const defaultValues = useMemo<ConnectionFormValues>(() => {
    return {
      name: '',
      description: '',
      folder_id: '',
      team_id: normalizeTeamValue(teamId),
      icon: defaultIcon ?? DEFAULT_CONNECTION_ICON_ID,
      color: '',
    }
  }, [defaultIcon, teamId])

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
    defaultValues,
  })

  useEffect(() => {
    if (open) {
      reset(defaultValues)
      setFormError(null)
      grantToggleInteractedRef.current = false
      setAutoGrantTeamPermissions(false)
      setSelectedIdentityId(null)
    }
  }, [defaultValues, open, reset])

  const selectedIcon = watch('icon')
  const selectedColor = watch('color')
  const selectedTeamValue = watch('team_id')

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

      if (effectiveTeamId && autoGrantTeamPermissions && missingTeamPermissionIds.length > 0) {
        const uniquePermissions = Array.from(new Set(missingTeamPermissionIds))
        if (uniquePermissions.length > 0) {
          payload.grant_team_permissions = uniquePermissions
        }
      }

      const connection = await create.mutateAsync(payload)
      onSuccess(connection)
      onClose()
    } catch (error) {
      const apiError = toApiError(error)
      setFormError(apiError)
    }
  }

  const isLoading = isSubmitting || create.isPending
  const folderOptions = useMemo(() => flattenFolders(folders), [folders])

  if (!protocol) {
    return null
  }

  const Icon = resolveProtocolIcon(protocol)

  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        title={`Configure ${protocol.name}`}
        description="Provide a name and optional folder to keep things organized."
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
                  <Select
                    value={(field.value && field.value.length ? field.value : '__unassigned__') as string}
                    onValueChange={(value) =>
                      field.onChange(value === '__unassigned__' ? '' : value)
                    }
                  >
                    <SelectTrigger
                      id="connection-folder"
                      className="h-10 w-full justify-between"
                    >
                      <SelectValue placeholder="Unassigned" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="__unassigned__">Unassigned</SelectItem>
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
                      value={(field.value && field.value.length ? field.value : '__personal__') as string}
                      onValueChange={(value) =>
                        field.onChange(value === '__personal__' ? '' : value)
                      }
                    >
                      <SelectTrigger
                        id="connection-team"
                        className="h-10 w-full justify-between"
                      >
                        <SelectValue placeholder="Personal workspace" />
                      </SelectTrigger>
                      <SelectContent align="start">
                        <SelectItem value="__personal__">Personal workspace</SelectItem>
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
                Create Connection
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
