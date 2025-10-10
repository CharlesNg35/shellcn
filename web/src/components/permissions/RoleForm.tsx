import { useMemo, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { roleCreateSchema, roleUpdateSchema } from '@/schemas/roles'
import type { RoleRecord } from '@/types/permission'
import type { RoleCreateSchema, RoleUpdateSchema } from '@/schemas/roles'
import { useRoleMutations } from '@/hooks/useRoles'
import type { ApiError } from '@/lib/api/http'

export type RoleFormMode = 'create' | 'edit'

interface RoleFormProps {
  mode?: RoleFormMode
  role?: RoleRecord
  onClose?: () => void
  onSuccess?: (role: RoleRecord) => void
}

type CreateFormValues = RoleCreateSchema
type UpdateFormValues = RoleUpdateSchema

type FormValues = CreateFormValues & Partial<UpdateFormValues>

export function RoleForm({ mode = 'create', role, onClose, onSuccess }: RoleFormProps) {
  const { createRole, updateRole } = useRoleMutations()
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const defaultValues = useMemo(() => {
    if (mode === 'create') {
      return {
        name: '',
        description: '',
      }
    }

    if (!role) {
      return {}
    }

    return {
      name: role.name,
      description: role.description ?? '',
    }
  }, [mode, role])

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(mode === 'create' ? roleCreateSchema : roleUpdateSchema) as any,
    defaultValues,
  })

  const handleError = (apiError: unknown) => {
    const err = apiError as ApiError
    setErrorMessage(err?.message || 'Unable to save role. Please try again.')
  }

  const handleSuccess = (result: RoleRecord) => {
    setErrorMessage(null)
    onSuccess?.(result)
    if (mode === 'create') {
      reset()
    }
    onClose?.()
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setErrorMessage(null)

    if (mode === 'create') {
      try {
        const created = await createRole.mutateAsync({
          name: values.name?.trim() ?? '',
          description: values.description?.trim() || undefined,
        })
        handleSuccess(created)
      } catch (err) {
        handleError(err)
      }
      return
    }

    if (!role) {
      setErrorMessage('Unable to load role context.')
      return
    }

    try {
      const updated = await updateRole.mutateAsync({
        roleId: role.id,
        payload: {
          name: role.is_system ? undefined : values.name?.trim(),
          description: values.description?.trim(),
        },
      })
      handleSuccess(updated)
    } catch (err) {
      handleError(err)
    }
  }

  const isSaving = isSubmitting || createRole.isPending || updateRole.isPending

  return (
    <form className="space-y-4" autoComplete="off" onSubmit={handleSubmit(onSubmit)}>
      <Input
        label="Role name"
        placeholder="Viewer"
        {...register('name')}
        error={errors.name?.message}
        disabled={mode === 'edit' && role?.is_system}
      />

      <Input
        label="Description"
        placeholder="Describe what this role can do"
        {...register('description')}
        error={errors.description?.message}
      />

      {errorMessage ? (
        <div className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMessage}
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        {onClose ? (
          <Button type="button" variant="outline" onClick={onClose} disabled={isSaving}>
            Cancel
          </Button>
        ) : null}
        <Button type="submit" loading={isSaving}>
          {mode === 'create' ? 'Create Role' : 'Save Changes'}
        </Button>
      </div>
    </form>
  )
}
