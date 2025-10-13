import { useMemo, useState, type FormEvent } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Modal } from '@/components/ui/Modal'
import { useUserMutations, useUser } from '@/hooks/useUsers'
import { usePermissions } from '@/hooks/usePermissions'
import type { UserRecord } from '@/types/users'
import { cn } from '@/lib/utils/cn'
import { PERMISSIONS } from '@/constants/permissions'
import { toast } from '@/lib/utils/toast'

interface UserDetailModalProps {
  userId?: string
  open: boolean
  onClose: () => void
  onEdit?: (user: UserRecord) => void
}

export function UserDetailModal({ userId, open, onClose, onEdit }: UserDetailModalProps) {
  const { data: user, isLoading } = useUser(userId ?? '', { enabled: open && Boolean(userId) })
  const { activate, deactivate, changePassword } = useUserMutations()
  const { hasAnyPermission } = usePermissions()
  const [passwordValue, setPasswordValue] = useState('')
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)

  const canUpdateUser = hasAnyPermission([
    PERMISSIONS.USER.UPDATE,
    PERMISSIONS.USER.EDIT,
    PERMISSIONS.USER.MANAGE,
  ])
  const canActivateUser = hasAnyPermission([
    PERMISSIONS.USER.ACTIVATE,
    PERMISSIONS.USER.EDIT,
    PERMISSIONS.USER.MANAGE,
  ])
  const canDeactivateUser = hasAnyPermission([
    PERMISSIONS.USER.DEACTIVATE,
    PERMISSIONS.USER.EDIT,
    PERMISSIONS.USER.MANAGE,
  ])
  const canResetPassword = hasAnyPermission([
    PERMISSIONS.USER.RESET_PASSWORD,
    PERMISSIONS.USER.EDIT,
    PERMISSIONS.USER.MANAGE,
  ])

  const fullName = useMemo(() => {
    if (!user) {
      return ''
    }
    const parts = [user.first_name, user.last_name].filter(Boolean)
    return parts.join(' ')
  }, [user])

  const handleToggleActive = async () => {
    if (!user) {
      return
    }
    if (user.is_root) {
      toast.info('Root account activation is managed from the Profile page.')
      return
    }
    if (user.is_active) {
      if (!canDeactivateUser) {
        toast.error('You do not have permission to deactivate users.')
        return
      }
      await deactivate.mutateAsync(user.id)
    } else {
      if (!canActivateUser) {
        toast.error('You do not have permission to activate users.')
        return
      }
      await activate.mutateAsync(user.id)
    }
  }

  const authProvider = user?.auth_provider?.toUpperCase() ?? 'LOCAL'
  const isExternal = Boolean(user?.auth_provider && user.auth_provider !== 'local')
  const isRootUser = Boolean(user?.is_root)

  const handlePasswordSubmit = async (event: FormEvent) => {
    event.preventDefault()
    if (!user || !passwordValue) {
      return
    }
    try {
      await changePassword.mutateAsync({ userId: user.id, password: passwordValue })
      setPasswordMessage('Password updated successfully')
      setPasswordValue('')
    } catch {
      setPasswordMessage('Failed to update password')
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={user ? `${user.username}${user.is_root ? ' • Root' : ''}` : 'User details'}
      description={fullName || user?.email}
      size="lg"
    >
      {isLoading ? (
        <div className="flex items-center justify-center py-10 text-sm text-muted-foreground">
          Loading user details...
        </div>
      ) : user ? (
        <div className="space-y-6">
          <div className="flex flex-col gap-4 rounded-lg border border-border/70 bg-muted/20 p-4">
            <div>
              <p className="text-sm font-semibold text-muted-foreground">Status</p>
              <Badge variant={user.is_active ? 'success' : 'secondary'} className="mt-2">
                {user.is_active ? 'Active' : 'Inactive'}
              </Badge>
            </div>
            <div>
              <p className="text-sm font-semibold text-muted-foreground">Auth Provider</p>
              <Badge variant={isExternal ? 'secondary' : 'outline'} className="mt-2">
                {authProvider}
              </Badge>
              {isExternal ? (
                <p className="mt-1 text-xs text-muted-foreground">
                  Profile attributes are managed by the upstream provider. You can still adjust
                  roles and account status here.
                </p>
              ) : null}
            </div>
            <div>
              <p className="text-sm font-semibold text-muted-foreground">Email</p>
              <p className="text-sm text-foreground">{user.email}</p>
            </div>
            {user.roles?.length ? (
              <div>
                <p className="text-sm font-semibold text-muted-foreground">Roles</p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {user.roles.map((role) => (
                    <Badge key={role.id} variant="outline">
                      {role.name}
                    </Badge>
                  ))}
                </div>
              </div>
            ) : null}
          </div>

          {(canUpdateUser || canActivateUser || canDeactivateUser) &&
            (isRootUser ? (
              <div className="rounded-md border border-border/70 bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
                The root account cannot be edited or disabled from the Users page. Manage it from
                your Profile instead.
              </div>
            ) : (
              <div className="flex flex-wrap gap-2">
                {canUpdateUser ? (
                  <Button variant="outline" onClick={() => onEdit?.(user)}>
                    {isExternal ? 'Manage access' : 'Edit user'}
                  </Button>
                ) : null}
                {(user.is_active ? canDeactivateUser : canActivateUser) ? (
                  <Button
                    variant={user.is_active ? 'secondary' : 'default'}
                    onClick={handleToggleActive}
                    loading={activate.isPending || deactivate.isPending}
                  >
                    {user.is_active ? 'Deactivate' : 'Activate'}
                  </Button>
                ) : null}
              </div>
            ))}

          {canResetPassword &&
            (isRootUser ? (
              <div className="rounded-md border border-border/70 bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
                Password updates for the root account are restricted to the Profile page.
              </div>
            ) : (
              <form className="space-y-3" autoComplete="off" onSubmit={handlePasswordSubmit}>
                <div className="space-y-2">
                  <p className="text-sm font-semibold text-muted-foreground">Password Management</p>
                  <Input
                    type="password"
                    placeholder="Set new password"
                    value={passwordValue}
                    onChange={(event) => setPasswordValue(event.target.value)}
                  />
                  {passwordMessage ? (
                    <p
                      className={cn(
                        'text-xs',
                        passwordMessage.includes('successfully')
                          ? 'text-emerald-500'
                          : 'text-destructive'
                      )}
                    >
                      {passwordMessage}
                    </p>
                  ) : null}
                </div>
                <div className="flex justify-end gap-2">
                  <Button type="button" variant="outline" onClick={() => setPasswordValue('')}>
                    Clear
                  </Button>
                  <Button
                    type="submit"
                    loading={changePassword.isPending}
                    disabled={!passwordValue}
                  >
                    Update Password
                  </Button>
                </div>
              </form>
            ))}
        </div>
      ) : (
        <div className="py-6 text-sm text-muted-foreground">User not found.</div>
      )}
    </Modal>
  )
}
