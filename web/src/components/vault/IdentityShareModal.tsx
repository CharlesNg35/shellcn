import { useMemo, useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { useIdentitySharing } from '@/hooks/useIdentities'
import { useUsers } from '@/hooks/useUsers'
import { useTeams } from '@/hooks/useTeams'
import type { IdentitySharePermission, IdentitySharePrincipalType } from '@/types/vault'

interface IdentityShareModalProps {
  identityId: string | undefined
  open: boolean
  onClose: () => void
}

const PERMISSION_OPTIONS: Array<{ label: string; value: IdentitySharePermission }> = [
  { label: 'Use (launch connections)', value: 'use' },
  { label: 'View metadata only', value: 'view_metadata' },
  { label: 'Edit identity', value: 'edit' },
]

export function IdentityShareModal({ identityId, open, onClose }: IdentityShareModalProps) {
  const { grant } = useIdentitySharing(identityId)
  const usersQuery = useUsers({ per_page: 50, status: 'all' }, { enabled: open })
  const teamsQuery = useTeams({ enabled: open })
  const users = useMemo(() => usersQuery.data?.data ?? [], [usersQuery.data])
  const teams = useMemo(() => teamsQuery.data?.data ?? [], [teamsQuery.data])

  const [principalType, setPrincipalType] = useState<IdentitySharePrincipalType>('user')
  const [principalId, setPrincipalId] = useState('')
  const [permission, setPermission] = useState<IdentitySharePermission>('use')
  const [expiresAt, setExpiresAt] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const principalOptions = useMemo(() => {
    if (principalType === 'user') {
      return users.map((user) => ({
        id: user.id,
        label: `${user.username}${user.email ? ` (${user.email})` : ''}`,
      }))
    }
    return teams.map((team) => ({ id: team.id, label: team.name }))
  }, [principalType, teams, users])

  const resetState = () => {
    setPrincipalId('')
    setPermission('use')
    setExpiresAt('')
    setFormError(null)
  }

  const handleClose = () => {
    if (grant.isPending) {
      return
    }
    onClose()
    resetState()
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!identityId) {
      return
    }
    if (!principalId) {
      setFormError('Please choose a target to share with')
      return
    }
    try {
      await grant.mutateAsync({
        principal_type: principalType,
        principal_id: principalId,
        permission,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      })
      handleClose()
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to share identity'
      setFormError(message)
    }
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Share identity"
      description="Grant access to this credential for a user or team."
    >
      <form className="space-y-6" onSubmit={handleSubmit}>
        <div className="space-y-2">
          <label className="text-sm font-medium">Share with</label>
          <div className="grid grid-cols-2 gap-2">
            <button
              type="button"
              className={`rounded-md border px-3 py-2 text-sm ${
                principalType === 'user'
                  ? 'border-primary bg-primary/5 text-primary'
                  : 'border-border bg-background text-foreground'
              }`}
              onClick={() => {
                setPrincipalType('user')
                setPrincipalId('')
              }}
            >
              User
            </button>
            <button
              type="button"
              className={`rounded-md border px-3 py-2 text-sm ${
                principalType === 'team'
                  ? 'border-primary bg-primary/5 text-primary'
                  : 'border-border bg-background text-foreground'
              }`}
              onClick={() => {
                setPrincipalType('team')
                setPrincipalId('')
              }}
            >
              Team
            </button>
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="share-principal">
            {principalType === 'user' ? 'User' : 'Team'}
          </label>
          <select
            id="share-principal"
            className="h-10 rounded-md border border-border bg-background px-3 text-sm"
            value={principalId}
            onChange={(event) => setPrincipalId(event.target.value)}
          >
            <option value="">Select {principalType === 'user' ? 'user' : 'team'}</option>
            {principalOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="share-permission">
            Permission
          </label>
          <select
            id="share-permission"
            className="h-10 rounded-md border border-border bg-background px-3 text-sm"
            value={permission}
            onChange={(event) => setPermission(event.target.value as IdentitySharePermission)}
          >
            {PERMISSION_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <p className="text-xs text-muted-foreground">
            “Use” permits launching connections with this credential, “Edit” grants full management.
          </p>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="share-expiry">
            Expiration (optional)
          </label>
          <Input
            id="share-expiry"
            type="datetime-local"
            value={expiresAt}
            onChange={(event) => setExpiresAt(event.target.value)}
          />
        </div>

        {formError ? <p className="text-sm text-destructive">{formError}</p> : null}

        <div className="flex justify-end gap-3">
          <Button type="button" variant="outline" onClick={handleClose} disabled={grant.isPending}>
            Cancel
          </Button>
          <Button type="submit" disabled={grant.isPending || !principalId}>
            {grant.isPending ? 'Sharing…' : 'Share identity'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}
