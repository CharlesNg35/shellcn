import { useEffect, useMemo, useState } from 'react'
import { z } from 'zod'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import type { Protocol } from '@/types/protocols'
import type { ConnectionFolderNode, ConnectionRecord } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { resolveProtocolIcon } from '@/lib/utils/protocolIcons'
import { useConnectionMutations } from '@/hooks/useConnectionMutations'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'

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

  const defaultValues = useMemo<ConnectionFormValues>(() => {
    return {
      name: '',
      description: '',
      folder_id: '',
      team_id: normalizeTeamValue(teamId),
    }
  }, [teamId])

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ConnectionFormValues>({
    resolver: zodResolver(connectionSchema),
    defaultValues,
  })

  useEffect(() => {
    if (open) {
      reset(defaultValues)
      setFormError(null)
    }
  }, [defaultValues, open, reset])

  const onSubmit: SubmitHandler<ConnectionFormValues> = async (values) => {
    setFormError(null)
    try {
      const connection = await create.mutateAsync({
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        protocol_id: protocol.id,
        folder_id: sanitizeId(values.folder_id),
        team_id: denormalizeTeamValue(values.team_id),
      })
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

          <div className="flex flex-col gap-2">
            <label className="text-sm font-medium text-foreground" htmlFor="connection-folder">
              Folder
            </label>
            <select
              id="connection-folder"
              className="h-10 rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              defaultValue=""
              {...register('folder_id')}
            >
              <option value="">Unassigned</option>
              {folderOptions.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="text-xs text-muted-foreground">
              Optional. Connections without a folder appear in the Unassigned view.
            </p>
          </div>

          {allowTeamAssignment && teams.length ? (
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium text-foreground" htmlFor="connection-team">
                Team
              </label>
              <select
                id="connection-team"
                className="h-10 rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                defaultValue={normalizeTeamValue(teamId)}
                {...register('team_id')}
              >
                <option value="">Personal workspace</option>
                {teams.map((team) => (
                  <option key={team.id} value={team.id}>
                    {team.name}
                  </option>
                ))}
              </select>
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
              Create Connection
            </Button>
          </div>
        </form>
      </div>
    </Modal>
  )
}

interface FolderOption {
  id: string
  label: string
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
