import { useEffect, useMemo, useState } from 'react'
import { Loader2, ShieldCheck } from 'lucide-react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { userCreateSchema, userUpdateSchema } from '@/schemas/users'
import type { UserRecord } from '@/types/users'
import { useUserMutations } from '@/hooks/useUsers'
import { useRoles } from '@/hooks/useRoles'
import { usePermissions } from '@/hooks/usePermissions'
import { ApiError, toApiError } from '@/lib/api/http'
import { PERMISSIONS } from '@/constants/permissions'

export type UserFormMode = 'create' | 'edit'

interface UserFormProps {
  mode?: UserFormMode
  user?: UserRecord
  onClose?: () => void
  onSuccess?: (user: UserRecord) => void
}

type CreateFormValues = z.infer<typeof userCreateSchema>
type UpdateFormValues = z.infer<typeof userUpdateSchema>

// Use a broader type that encompasses both schemas
type FormValues = CreateFormValues & Partial<UpdateFormValues>

function toSet(values: string[]): Set<string> {
  return new Set(values)
}

function selectionsMatch(selection: string[], baseline: Set<string>): boolean {
  if (selection.length !== baseline.size) {
    return false
  }
  for (const value of selection) {
    if (!baseline.has(value)) {
      return false
    }
  }
  return true
}

export function UserForm({ mode = 'create', user, onClose, onSuccess }: UserFormProps) {
  const [formError, setFormError] = useState<ApiError | null>(null)
  const { create, update, setRoles } = useUserMutations()
  const { data: availableRoles, isLoading: isRolesLoading } = useRoles()
  const { hasPermission } = usePermissions()
  const canManageRoles = hasPermission(PERMISSIONS.PERMISSION.MANAGE)

  const assignedRoleIds = useMemo(() => user?.roles?.map((role) => role.id) ?? [], [user?.roles])
  const baselineRoleSet = useMemo(() => toSet(assignedRoleIds), [assignedRoleIds])
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>(assignedRoleIds)

  useEffect(() => {
    setSelectedRoleIds(assignedRoleIds)
  }, [assignedRoleIds])

  const rolesChanged = useMemo(() => {
    if (mode === 'create') {
      return selectedRoleIds.length > 0
    }
    return !selectionsMatch(selectedRoleIds, baselineRoleSet)
  }, [mode, selectedRoleIds, baselineRoleSet])

  const sortedRoles = useMemo(() => {
    return [...(availableRoles ?? [])].sort((a, b) => a.name.localeCompare(b.name))
  }, [availableRoles])

  const toggleRole = (roleId: string) => {
    setSelectedRoleIds((current) => {
      if (current.includes(roleId)) {
        return current.filter((id) => id !== roleId)
      }
      return [...current, roleId]
    })
  }

  const isRoleMutationPending = setRoles.isPending

  const defaultValues = useMemo(() => {
    if (mode === 'create') {
      return {
        username: '',
        email: '',
        password: '',
        first_name: '',
        last_name: '',
        is_active: true,
        is_root: false,
      }
    }
    if (!user) {
      return {}
    }
    return {
      username: user.username,
      email: user.email,
      first_name: user.first_name ?? '',
      last_name: user.last_name ?? '',
      avatar: user.avatar ?? '',
      is_active: user.is_active,
    }
  }, [mode, user])

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(mode === 'create' ? userCreateSchema : userUpdateSchema) as any,
    defaultValues,
  })

  const handleError = (apiError: unknown) => {
    const err = toApiError(apiError)
    setFormError(err)
  }

  const applyRoleChanges = async (currentUser: UserRecord): Promise<UserRecord | null> => {
    if (!canManageRoles || !rolesChanged) {
      return currentUser
    }

    try {
      const updatedUser = await setRoles.mutateAsync({
        userId: currentUser.id,
        roleIds: selectedRoleIds,
      })
      return updatedUser
    } catch (apiError) {
      handleError(apiError)
      return null
    }
  }

  const savingInProgress =
    isSubmitting || create.isPending || update.isPending || isRoleMutationPending
  const roleSelectionDisabled = !canManageRoles || savingInProgress

  const isExternalUser =
    mode === 'edit' && Boolean(user?.auth_provider && user.auth_provider !== 'local')

  const handleSuccess = (result: UserRecord) => {
    setFormError(null)
    onSuccess?.(result)
    if (mode === 'create') {
      reset()
    }
    onClose?.()
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setFormError(null)

    if (mode === 'create') {
      try {
        const created = await create.mutateAsync({
          username: values.username!,
          email: values.email!,
          password: values.password!,
          first_name: values.first_name,
          last_name: values.last_name,
          avatar: values.avatar,
          is_root: values.is_root,
          is_active: values.is_active,
        })
        const withRoles = await applyRoleChanges(created)
        if (!withRoles) {
          return
        }
        handleSuccess(withRoles)
      } catch (apiError) {
        handleError(apiError)
      }
      return
    }

    if (!user) {
      setFormError(
        new ApiError({
          code: 'USER_CONTEXT_MISSING',
          message: 'Missing user context for update.',
        })
      )
      return
    }

    try {
      const updated = await update.mutateAsync({
        userId: user.id,
        payload: {
          username: values.username,
          email: values.email,
          first_name: values.first_name,
          last_name: values.last_name,
          avatar: values.avatar,
        },
      })
      const withRoles = await applyRoleChanges(updated)
      if (!withRoles) {
        return
      }
      handleSuccess(withRoles)
    } catch (apiError) {
      handleError(apiError)
    }
  }

  return (
    <form className="space-y-4" autoComplete="off" onSubmit={handleSubmit(onSubmit)}>
      {isExternalUser ? (
        <div className="rounded-lg border border-border/70 bg-muted/10 p-3 text-sm text-muted-foreground">
          This account is managed by the{' '}
          <span className="font-medium">{user?.auth_provider?.toUpperCase()}</span> identity
          provider. Profile details are read-only, but you can adjust activation status or role
          assignments.
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="Username"
          placeholder="username"
          {...register('username')}
          error={errors.username?.message}
          disabled={isExternalUser}
        />
        <Input
          type="email"
          label="Email"
          placeholder="user@example.com"
          {...register('email')}
          error={errors.email?.message}
          disabled={isExternalUser}
        />
      </div>

      {mode === 'create' ? (
        <Input
          type="password"
          label="Password"
          placeholder="Minimum 8 characters"
          {...register('password')}
          error={errors.password?.message}
        />
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="First name"
          {...register('first_name')}
          error={errors.first_name?.message}
          disabled={isExternalUser}
        />
        <Input
          label="Last name"
          {...register('last_name')}
          error={errors.last_name?.message}
          disabled={isExternalUser}
        />
      </div>

      <Input
        label="Avatar URL"
        {...register('avatar')}
        error={errors.avatar?.message}
        disabled={isExternalUser}
      />

      <div className="space-y-3 rounded-lg border border-border/70 bg-muted/10 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-sm font-semibold text-foreground">Roles</p>
            <p className="text-xs text-muted-foreground">
              Assign roles to control {mode === 'create' ? 'the new' : 'this'} user's permissions.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="text-[10px] uppercase tracking-wide">
              {canManageRoles ? 'Manage' : 'Read only'}
            </Badge>
            <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
              {selectedRoleIds.length} selected
            </Badge>
          </div>
        </div>

        {isRolesLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" /> Loading roles…
          </div>
        ) : sortedRoles.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No roles available yet. Create roles from the Permissions page first.
          </p>
        ) : (
          <div className="max-h-56 space-y-2 overflow-y-auto pr-1">
            {sortedRoles.map((role) => {
              const checked = selectedRoleIds.includes(role.id)
              return (
                <label
                  key={role.id}
                  className="flex cursor-pointer items-start gap-3 rounded-lg border border-border/60 bg-background px-3 py-2 hover:border-border"
                >
                  <Checkbox
                    checked={checked}
                    onCheckedChange={() => toggleRole(role.id)}
                    disabled={roleSelectionDisabled}
                  />
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-medium text-foreground">{role.name}</span>
                      {role.is_system ? (
                        <Badge variant="outline" className="flex items-center gap-1 text-[10px]">
                          <ShieldCheck className="h-3 w-3" /> System
                        </Badge>
                      ) : null}
                      {checked ? (
                        <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                          Assigned
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {role.description || 'No description provided.'}
                    </p>
                  </div>
                </label>
              )
            })}
          </div>
        )}

        {!canManageRoles ? (
          <p className="text-xs text-muted-foreground">
            You do not have permission to change role assignments.
          </p>
        ) : null}
      </div>

      {formError ? (
        <div className="rounded border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <p className="font-medium">{formError.message}</p>
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        {onClose ? (
          <Button type="button" variant="outline" onClick={onClose} disabled={savingInProgress}>
            Cancel
          </Button>
        ) : null}
        <Button type="submit" loading={savingInProgress}>
          {mode === 'create' ? 'Create User' : 'Save Changes'}
        </Button>
      </div>
    </form>
  )
}
