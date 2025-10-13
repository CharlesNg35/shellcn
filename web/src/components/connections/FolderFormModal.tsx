import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import type { ConnectionFolderSummary } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { useConnectionFolderMutations } from '@/hooks/useConnectionFolderMutations'
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
  team_id: z.string().trim().optional().or(z.literal('')),
})

export type FolderFormMode = 'create' | 'edit'

interface FolderFormModalProps {
  open: boolean
  mode: FolderFormMode
  onClose: () => void
  onSuccess: (folder: ConnectionFolderSummary) => void
  folder?: ConnectionFolderSummary
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
        team_id: folder.team_id ?? '',
      }
    }
    return {
      name: '',
      description: '',
      team_id: normalizeTeamValue(teamId),
    }
  }, [folder, mode, teamId])

  const {
    handleSubmit,
    register,
    reset,
    control,
    formState: { errors, isSubmitting },
  } = useForm<FolderFormValues>({
    resolver: zodResolver(folderSchema),
    defaultValues,
  })

  useEffect(() => {
    if (open) {
      reset(defaultValues)
      setFormError(null)
    }
  }, [defaultValues, open, reset])

  const onSubmit: SubmitHandler<FolderFormValues> = async (values) => {
    setFormError(null)
    const payload = {
      name: values.name.trim(),
      description: values.description?.trim() || undefined,
      parent_id: mode === 'edit' ? (folder?.parent_id ?? null) : null,
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
      ? 'Name your folder to group related connections for your team or personal workspace.'
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

          {allowTeamAssignment ? (
            <div className="grid gap-2">
              <label className="text-sm font-medium text-foreground" htmlFor="folder-team">
                Assign to team
              </label>
              <Controller
                name="team_id"
                control={control}
                render={({ field }) => (
                  <Select value={field.value ?? ''} onValueChange={field.onChange}>
                    <SelectTrigger id="folder-team" className="h-10 w-full justify-between">
                      <SelectValue placeholder="Personal workspace" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      <SelectItem value="">Personal workspace</SelectItem>
                      {teams.length === 0 ? (
                        <SelectItem value="__no_team__" disabled>
                          Create a team to share this folder
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
