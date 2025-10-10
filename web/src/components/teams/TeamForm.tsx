import { useEffect, useMemo, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Button } from '@/components/ui/Button'
import { teamCreateSchema, teamUpdateSchema } from '@/schemas/teams'
import type { TeamRecord } from '@/types/teams'
import { useTeamMutations } from '@/hooks/useTeams'
import type { ApiError } from '@/lib/api/http'

export type TeamFormMode = 'create' | 'edit'

interface TeamFormProps {
  mode?: TeamFormMode
  team?: TeamRecord
  onClose?: () => void
  onSuccess?: (team: TeamRecord) => void
}

type CreateFormValues = z.infer<typeof teamCreateSchema>
type UpdateFormValues = z.infer<typeof teamUpdateSchema>

type FormValues = CreateFormValues & Partial<UpdateFormValues>

export function TeamForm({ mode = 'create', team, onClose, onSuccess }: TeamFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const { create, update } = useTeamMutations()

  const defaultValues = useMemo(() => {
    if (mode === 'create') {
      return {
        name: '',
        description: '',
      }
    }
    if (!team) {
      return {}
    }
    return {
      name: team.name,
      description: team.description ?? '',
    }
  }, [mode, team])

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<FormValues>({
    resolver: zodResolver(mode === 'create' ? teamCreateSchema : teamUpdateSchema) as never,
    defaultValues,
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const handleError = (error: unknown) => {
    const apiError = error as ApiError | undefined
    if (apiError?.message) {
      setErrorMessage(apiError.message)
      return
    }
    setErrorMessage('Unable to save team. Please try again.')
  }

  const handleSuccess = (result: TeamRecord) => {
    setErrorMessage(null)
    onSuccess?.(result)
    if (mode === 'create') {
      reset({
        name: '',
        description: '',
      })
    }
    onClose?.()
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setErrorMessage(null)

    if (mode === 'create') {
      try {
        const created = await create.mutateAsync({
          name: values.name!,
          description: values.description,
        })
        handleSuccess(created)
      } catch (error) {
        handleError(error)
      }
      return
    }

    if (!team) {
      setErrorMessage('Team context is missing for update.')
      return
    }

    try {
      const updated = await update.mutateAsync({
        teamId: team.id,
        payload: {
          name: values.name,
          description: values.description,
        },
      })
      handleSuccess(updated)
    } catch (error) {
      handleError(error)
    }
  }

  const isLoading = isSubmitting || create.isPending || update.isPending

  return (
    <form className="space-y-4" autoComplete="off" onSubmit={handleSubmit(onSubmit)}>
      <Input
        label="Team name"
        placeholder="Security Operations"
        {...register('name')}
        error={errors.name?.message}
      />

      <Textarea
        label="Description"
        placeholder="Describe the team's responsibilities and scope"
        rows={4}
        {...register('description')}
        error={errors.description?.message}
        helpText="Optional — maximum 512 characters"
      />

      {errorMessage ? (
        <div className="rounded border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMessage}
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        {onClose ? (
          <Button type="button" variant="outline" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
        ) : null}
        <Button type="submit" loading={isLoading}>
          {mode === 'create' ? 'Create Team' : 'Save Changes'}
        </Button>
      </div>
    </form>
  )
}
