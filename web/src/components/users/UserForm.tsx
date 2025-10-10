import { useMemo, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { userCreateSchema, userUpdateSchema } from '@/schemas/users'
import type { UserRecord } from '@/types/users'
import { useUserMutations } from '@/hooks/useUsers'
import type { ApiError } from '@/lib/api/http'

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

export function UserForm({ mode = 'create', user, onClose, onSuccess }: UserFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const { create, update } = useUserMutations()

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
    const err = apiError as ApiError
    if (err?.message) {
      setErrorMessage(err.message)
    } else {
      setErrorMessage('Unable to save user. Please try again.')
    }
  }

  const handleSuccess = (result: UserRecord) => {
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
        handleSuccess(created)
      } catch (apiError) {
        handleError(apiError)
      }
      return
    }

    if (!user) {
      setErrorMessage('Missing user context for update.')
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
      handleSuccess(updated)
    } catch (apiError) {
      handleError(apiError)
    }
  }

  return (
    <form className="space-y-4" autoComplete="off" onSubmit={handleSubmit(onSubmit)}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Input
          label="Username"
          placeholder="username"
          {...register('username')}
          error={errors.username?.message}
        />
        <Input
          type="email"
          label="Email"
          placeholder="user@example.com"
          {...register('email')}
          error={errors.email?.message}
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
        <Input label="First name" {...register('first_name')} error={errors.first_name?.message} />
        <Input label="Last name" {...register('last_name')} error={errors.last_name?.message} />
      </div>

      <Input label="Avatar URL" {...register('avatar')} error={errors.avatar?.message} />

      {errorMessage ? (
        <div className="rounded border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMessage}
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        {onClose ? (
          <Button type="button" variant="outline" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
        ) : null}
        <Button type="submit" loading={isSubmitting || create.isPending || update.isPending}>
          {mode === 'create' ? 'Create User' : 'Save Changes'}
        </Button>
      </div>
    </form>
  )
}
