import { useEffect, useMemo, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import type { ConnectionFolderSummary } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { useConnectionFolderMutations } from '@/hooks/useConnectionFolderMutations'
import { FOLDER_CONFIG } from '@/config/folders'
import { DEFAULT_FOLDER_ICON_ID, FOLDER_COLOR_OPTIONS, FOLDER_ICON_OPTIONS } from '@/constants/folders'
import { cn } from '@/lib/utils/cn'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'

const folderSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Folder name is required')
    .max(255, 'Folder name must be 255 characters or less'),
  description: z
    .string()
    .trim()
    .max(1000, 'Description must be 1000 characters or less')
    .optional()
    .or(z.literal('')),
  icon: z
    .string()
    .trim()
    .max(64, 'Icon identifier must be 64 characters or less')
    .optional()
    .or(z.literal('')),
  color: z
    .string()
    .trim()
    .max(32, 'Color identifier must be 32 characters or less')
    .optional()
    .or(z.literal('')),
  team_id: z.string().trim().optional().or(z.literal('')),
})

export type FolderFormMode = 'create' | 'edit'

interface FolderFormModalProps {
  open: boolean
  mode: FolderFormMode
  onClose: () => void
  onSuccess: (folder: ConnectionFolderSummary) => void
  folder?: ConnectionFolderSummary
  parentFolder?: ConnectionFolderSummary | null
  teamId?: string | null
  teams?: TeamRecord[]
  allowTeamAssignment?: boolean
}

type FolderFormValues = z.infer<typeof folderSchema>

export function FolderFormModal({
  open,
  mode,
  onClose,
  onSuccess,
  folder,
  parentFolder,
  teamId,
  teams = [],
  allowTeamAssignment = false,
}: FolderFormModalProps) {
  const { create, update } = useConnectionFolderMutations()
  const [formError, setFormError] = useState<ApiError | null>(null)

  const defaultValues = useMemo<FolderFormValues>(() => {
    if (mode === 'edit' && folder) {
      return {
        name: folder.name,
        description: folder.description ?? '',
        icon: folder.icon ?? '',
        color: folder.color ?? '',
        team_id: folder.team_id ?? '',
      }
    }
    return {
      name: '',
      description: '',
      icon: parentFolder?.icon ?? DEFAULT_FOLDER_ICON_ID,
      color: parentFolder?.color ?? '',
      team_id: normalizeTeamValue(teamId),
    }
  }, [folder, mode, parentFolder?.color, parentFolder?.icon, teamId])

  const {
    handleSubmit,
    register,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FolderFormValues>({
    resolver: zodResolver(folderSchema),
    defaultValues,
  })

  useEffect(() => {
    if (open) {
      reset(defaultValues)
    }
  }, [defaultValues, open, reset])

  const selectedIcon = watch('icon')
  const selectedColor = watch('color')

  useEffect(() => {
    if (!selectedIcon) {
      setValue('icon', DEFAULT_FOLDER_ICON_ID, { shouldValidate: false })
    }
  }, [selectedIcon, setValue])

  const onSubmit: SubmitHandler<FolderFormValues> = async (values) => {
    setFormError(null)
    const payload = {
      name: values.name.trim(),
      description: values.description?.trim() || undefined,
      icon: values.icon?.trim() || undefined,
      color: values.color?.trim() || undefined,
      parent_id: FOLDER_CONFIG.allowParentSelection ? parentFolder?.id ?? null : null,
      team_id: denormalizeTeamValue(values.team_id),
    }

    try {
      if (mode === 'create') {
        const created = await create.mutateAsync(payload)
        onSuccess(created)
        onClose()
        return
      }

      if (!folder) {
        throw new Error('Folder context missing for edit')
      }

      const updated = await update.mutateAsync({
        id: folder.id,
        payload,
      })
      onSuccess(updated)
      onClose()
    } catch (error) {
      const apiError = toApiError(error)
      setFormError(apiError)
    }
  }

  const isLoading = isSubmitting || create.isPending || update.isPending

  const title = mode === 'create' ? 'Create Folder' : `Edit "${folder?.name ?? ''}"`
  const description =
    mode === 'create'
      ? 'Name your folder and choose an icon to personalize your workspace.'
      : 'Update folder details to keep your workspace organized.'

  return (
    <Modal open={open} onClose={onClose} title={title} description={description} size="lg">
      <form className="space-y-6" onSubmit={handleSubmit(onSubmit)} autoComplete="off">
        <div className="grid gap-4">
          <Input
            label="Folder name"
            placeholder="Production Servers"
            {...register('name')}
            error={errors.name?.message}
          />

          <Textarea
            label="Description"
            placeholder="Optional - describe what belongs in this folder."
            rows={3}
            {...register('description')}
            error={errors.description?.message}
          />

          <div className="grid gap-3">
            <label className="text-sm font-medium text-foreground">Icon</label>
            <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
              {FOLDER_ICON_OPTIONS.map(({ id, label, icon: Icon }) => {
                const isActive = (selectedIcon || DEFAULT_FOLDER_ICON_ID) === id
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
                    <Icon className="h-4 w-4" />
                    <span className="truncate">{label}</span>
                  </button>
                )
              })}
            </div>
          </div>

          <div className="grid gap-3">
            <label className="text-sm font-medium text-foreground">Color</label>
            <div className="flex flex-wrap gap-2">
              <ColorSwatch
                key="none"
                label="Default"
                color=""
                isActive={!selectedColor}
                onSelect={() => setValue('color', '', { shouldValidate: true })}
              />
              {FOLDER_COLOR_OPTIONS.map((option) => (
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

          {allowTeamAssignment ? (
            <div className="grid gap-2">
              <label className="text-sm font-medium text-foreground" htmlFor="folder-team">
                Assign to team
              </label>
              <select
                id="folder-team"
                className="h-10 rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={watch('team_id') ?? ''}
                onChange={(event) => setValue('team_id', event.target.value, { shouldValidate: true })}
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
            {mode === 'create' ? 'Create Folder' : 'Save Changes'}
          </Button>
        </div>
      </form>
    </Modal>
  )
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

function normalizeTeamValue(teamId?: string | null) {
  if (!teamId) {
    return ''
  }
  if (teamId === 'personal') {
    return ''
  }
  return teamId
}

function denormalizeTeamValue(teamId?: string | null) {
  const trimmed = teamId?.trim()
  if (!trimmed) {
    return null
  }
  if (trimmed === 'personal') {
    return null
  }
  return trimmed
}
