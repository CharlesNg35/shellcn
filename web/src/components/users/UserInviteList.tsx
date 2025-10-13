import { formatDistanceToNow } from 'date-fns'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import type { InviteRecord } from '@/types/invites'

interface UserInviteListProps {
  invites: InviteRecord[] | undefined
  isLoading: boolean
  onRevoke: (inviteId: string) => void
  isRevoking: (inviteId: string) => boolean
}

export function UserInviteList({ invites, isLoading, onRevoke, isRevoking }: UserInviteListProps) {
  if (isLoading) {
    return <Skeleton className="h-32 w-full" />
  }

  if (!invites?.length) {
    return <p className="text-sm text-muted-foreground">No invitations found.</p>
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card">
      <table className="min-w-full divide-y divide-border text-sm">
        <thead className="bg-muted/40">
          <tr>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">Email</th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">Team</th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">Status</th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">Expires</th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {invites.map((invite) => {
            const expiresIn = invite.expires_at
              ? formatDistanceToNow(new Date(invite.expires_at), { addSuffix: true })
              : '—'

            return (
              <tr key={invite.id}>
                <td className="px-4 py-3 font-medium text-foreground">{invite.email}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {invite.team_name ?? invite.team_id ?? '—'}
                </td>
                <td className="px-4 py-3 capitalize text-muted-foreground">{invite.status}</td>
                <td className="px-4 py-3 text-muted-foreground">{expiresIn}</td>
                <td className="px-4 py-3">
                  {invite.status === 'pending' ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onRevoke(invite.id)}
                      loading={isRevoking(invite.id)}
                    >
                      Revoke
                    </Button>
                  ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
