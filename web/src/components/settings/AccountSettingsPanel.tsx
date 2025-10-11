import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useShallow } from 'zustand/react/shallow'
import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { useProfileSettings } from '@/hooks/useProfileSettings'
import { profileUpdateSchema, type ProfileUpdateFormValues } from '@/schemas/profile'
import { useAuthStore } from '@/store/auth-store'
import type { ProfileUpdatePayload } from '@/types/profile'

interface AccountSettingsPanelProps {
  className?: string
}

export function AccountSettingsPanel({ className }: AccountSettingsPanelProps) {
  const { user } = useAuthStore(useShallow((state) => ({ user: state.user })))
  const { updateProfile } = useProfileSettings()

  const defaultValues = useMemo<ProfileUpdateFormValues>(() => {
    return {
      username: user?.username ?? '',
      email: user?.email ?? '',
      first_name: user?.first_name ?? '',
      last_name: user?.last_name ?? '',
      avatar: user?.avatar ?? '',
    }
  }, [user])

  const {
    register,
    handleSubmit,
    formState: { errors, isDirty },
    reset,
  } = useForm<ProfileUpdateFormValues>({
    resolver: zodResolver(profileUpdateSchema),
    defaultValues,
    mode: 'onBlur',
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  const isExternalProvider = useMemo(() => {
    const provider = user?.auth_provider?.toLowerCase()
    return provider ? provider !== 'local' : false
  }, [user?.auth_provider])

  const onSubmit = (values: ProfileUpdateFormValues) => {
    if (!user) {
      return
    }
    if (isExternalProvider) {
      return
    }

    const payload: ProfileUpdatePayload = {
      username: values.username.trim(),
      email: values.email.trim(),
      first_name: values.first_name?.trim() || undefined,
      last_name: values.last_name?.trim() || undefined,
      avatar: values.avatar?.trim() || undefined,
    }

    updateProfile.mutate(payload)
  }

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
          <CardDescription>Manage your account identity and contact information.</CardDescription>
        </CardHeader>
        <CardContent>
          {user ? (
            <form className="space-y-6" onSubmit={handleSubmit(onSubmit)}>
              {isExternalProvider ? (
                <div className="rounded-md border border-border/60 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
                  Profile fields are synchronized by your identity provider. You can view details
                  here, but changes must be made upstream.
                </div>
              ) : null}

              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Input
                  label="Username"
                  placeholder="username"
                  {...register('username')}
                  error={errors.username?.message}
                  disabled={isExternalProvider || updateProfile.isPending}
                />
                <Input
                  type="email"
                  label="Email"
                  placeholder="user@example.com"
                  {...register('email')}
                  error={errors.email?.message}
                  disabled={isExternalProvider || updateProfile.isPending}
                />
              </div>

              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Input
                  label="First name"
                  placeholder="Optional"
                  {...register('first_name')}
                  error={errors.first_name?.message}
                  disabled={isExternalProvider || updateProfile.isPending}
                />
                <Input
                  label="Last name"
                  placeholder="Optional"
                  {...register('last_name')}
                  error={errors.last_name?.message}
                  disabled={isExternalProvider || updateProfile.isPending}
                />
              </div>

              <Input
                label="Avatar URL"
                placeholder="https://example.com/avatar.png"
                {...register('avatar')}
                error={errors.avatar?.message}
                disabled={isExternalProvider || updateProfile.isPending}
              />

              <div className="flex justify-end">
                <Button
                  type="submit"
                  loading={updateProfile.isPending}
                  disabled={isExternalProvider || (!isDirty && !updateProfile.isPending)}
                >
                  Save changes
                </Button>
              </div>
            </form>
          ) : (
            <p className="text-sm text-muted-foreground">Loading profile information…</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
