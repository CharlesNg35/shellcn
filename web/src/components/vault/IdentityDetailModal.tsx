import { formatDistanceToNow } from 'date-fns'
import { Shield, User, Users, Zap } from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { IdentityScopeBadge } from '@/components/vault/IdentityScopeBadge'
import { useIdentity, useIdentitySharing } from '@/hooks/useIdentities'
import type { IdentityRecord } from '@/types/vault'

interface IdentityDetailModalProps {
  identityId: string | undefined
  open: boolean
  onClose: () => void
  onEditIdentity?: (identity: IdentityRecord) => void
  onShareIdentity?: (identity: IdentityRecord) => void
}

function formatDate(value?: string | null) {
  if (!value) {
    return 'Never'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Never'
  }
  return `${date.toLocaleString()} (${formatDistanceToNow(date, { addSuffix: true })})`
}

export function IdentityDetailModal({
  identityId,
  open,
  onClose,
  onEditIdentity,
  onShareIdentity,
}: IdentityDetailModalProps) {
  const identityQuery = useIdentity(identityId, { enabled: open && Boolean(identityId) })
  const { revoke } = useIdentitySharing(identityId)

  const identity = identityQuery.data

  const handleClose = () => {
    if (revoke.isPending) {
      return
    }
    onClose()
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      size="2xl"
      title="Identity details"
      description="Review credential metadata, usage statistics, and share access."
    >
      {identityQuery.isLoading || !identity ? (
        <div className="space-y-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-32 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
      ) : (
        <div className="space-y-6">
          <header className="flex flex-col gap-4 border-b border-border pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="space-y-1">
              <h2 className="text-xl font-semibold text-foreground">{identity.name}</h2>
              <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                <IdentityScopeBadge scope={identity.scope} />
                <span className="inline-flex items-center gap-1">
                  <Shield className="h-4 w-4" />
                  Owner: {identity.owner_user_id}
                </span>
                {identity.template_id ? (
                  <span className="inline-flex items-center gap-1">
                    <Zap className="h-4 w-4" />
                    Template: {identity.template_id}
                  </span>
                ) : null}
              </div>
              {identity.description ? (
                <p className="text-sm text-muted-foreground">{identity.description}</p>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => onShareIdentity?.(identity)}>
                Share
              </Button>
              <Button onClick={() => onEditIdentity?.(identity)}>Edit</Button>
            </div>
          </header>

          <section className="grid gap-4 rounded-lg border border-border bg-muted/20 p-4 sm:grid-cols-3">
            <div>
              <p className="text-xs uppercase text-muted-foreground">Usage count</p>
              <p className="text-lg font-semibold text-foreground">{identity.usage_count}</p>
              <p className="text-xs text-muted-foreground">
                Last used: {formatDate(identity.last_used_at)}
              </p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Connections</p>
              <p className="text-lg font-semibold text-foreground">{identity.connection_count}</p>
              <p className="text-xs text-muted-foreground">
                Created: {formatDate(identity.created_at)}
              </p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Rotation</p>
              <p className="text-lg font-semibold text-foreground">
                {identity.last_rotated_at ? formatDate(identity.last_rotated_at) : 'Never rotated'}
              </p>
            </div>
          </section>

          <section className="space-y-3">
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-muted-foreground" />
              <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Shares
              </h3>
            </div>
            {identity.shares && identity.shares.length > 0 ? (
              <div className="space-y-2">
                {identity.shares.map((share) => (
                  <div
                    key={share.id}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border bg-card/60 p-3"
                  >
                    <div className="space-y-1">
                      <p className="text-sm font-medium text-foreground">
                        {share.principal_type === 'user' ? (
                          <User className="mr-2 inline h-4 w-4" />
                        ) : (
                          <Users className="mr-2 inline h-4 w-4" />
                        )}
                        {share.principal_type}: {share.principal_id}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        Permission: {share.permission} · Granted by {share.granted_by}
                      </p>
                      {share.expires_at ? (
                        <p className="text-xs text-muted-foreground">
                          Expires: {formatDate(share.expires_at)}
                        </p>
                      ) : null}
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={revoke.isPending}
                      onClick={() => revoke.mutateAsync(share.id)}
                    >
                      Revoke
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">This identity is not shared yet.</p>
            )}
          </section>

          <section className="space-y-3">
            <div className="flex items-center gap-2">
              <Zap className="h-4 w-4 text-muted-foreground" />
              <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Activity timeline
              </h3>
            </div>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li>
                <span className="font-medium text-foreground">Created:</span>{' '}
                {formatDate(identity.created_at)}
              </li>
              <li>
                <span className="font-medium text-foreground">Last used:</span>{' '}
                {formatDate(identity.last_used_at)}
              </li>
              <li>
                <span className="font-medium text-foreground">Last rotated:</span>{' '}
                {identity.last_rotated_at ? formatDate(identity.last_rotated_at) : 'Never'}
              </li>
            </ul>
          </section>
        </div>
      )}
    </Modal>
  )
}
