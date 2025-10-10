import { useMemo, useState, type FormEvent } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { useUserMutations, useUser } from '@/hooks/useUsers'
import type { UserRecord } from '@/types/users'
import { cn } from '@/lib/utils/cn'
import { PERMISSIONS } from '@/constants/permissions'

interface UserDetailModalProps {
  userId?: string
  open: boolean
  onClose: () => void
  onEdit?: (user: UserRecord) => void
}

export function UserDetailModal({ userId, open, onClose, onEdit }: UserDetailModalProps) {
  const { data: user, isLoading } = useUser(userId ?? '', { enabled: open && Boolean(userId) })
  const { activate, deactivate, changePassword } = useUserMutations()
  const [passwordValue, setPasswordValue] = useState('')
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)

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
    if (user.is_active) {
      await deactivate.mutateAsync(user.id)
    } else {
      await activate.mutateAsync(user.id)
    }
  }

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

          <PermissionGuard permission={PERMISSIONS.USER.EDIT}>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => onEdit?.(user)}>
                Edit user
              </Button>
              <Button
                variant={user.is_active ? 'secondary' : 'default'}
                onClick={handleToggleActive}
                loading={activate.isPending || deactivate.isPending}
                disabled={user.is_root && !user.is_active}
              >
                {user.is_active ? 'Deactivate' : 'Activate'}
              </Button>
            </div>
          </PermissionGuard>

          <PermissionGuard permission={PERMISSIONS.USER.EDIT}>
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
                <Button type="submit" loading={changePassword.isPending} disabled={!passwordValue}>
                  Update Password
                </Button>
              </div>
            </form>
          </PermissionGuard>
        </div>
      ) : (
        <div className="py-6 text-sm text-muted-foreground">User not found.</div>
      )}
    </Modal>
  )
}
