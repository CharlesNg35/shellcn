import { useMemo } from 'react'
import { formatDistanceToNow } from 'date-fns'
import { Users, UserPlus, Key } from 'lucide-react'

import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import type { ActiveConnectionSession, ActiveSessionParticipant } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

interface SshWorkspaceHeaderProps {
  session: ActiveConnectionSession
  participants?: Record<string, ActiveSessionParticipant>
  currentUserId?: string
  onOpenShare?: () => void
  canShare?: boolean
  className?: string
}

export function SshWorkspaceHeader({
  session,
  participants,
  currentUserId,
  onOpenShare,
  canShare,
  className,
}: SshWorkspaceHeaderProps) {
  const startedAt = session.started_at ? new Date(session.started_at) : undefined
  const lastSeenAt = session.last_seen_at ? new Date(session.last_seen_at) : undefined
  const participantList = useMemo(() => {
    if (!participants) {
      return []
    }
    const ownerId = session.owner_user_id ?? ''
    const writeHolderId = session.write_holder ?? ''
    return Object.values(participants)
      .map((participant) => ({
        ...participant,
        is_owner: participant.is_owner ?? participant.user_id === ownerId,
        is_write_holder: participant.is_write_holder ?? participant.user_id === writeHolderId,
        joinedAt: participant.joined_at ? new Date(participant.joined_at) : undefined,
      }))
      .sort((a, b) => {
        if (!a.joinedAt || !b.joinedAt) {
          return 0
        }
        return a.joinedAt.getTime() - b.joinedAt.getTime()
      })
  }, [participants, session.owner_user_id, session.write_holder])

  const isOwner =
    currentUserId && session.owner_user_id ? session.owner_user_id === currentUserId : false

  const shareButtonVisible = Boolean(onOpenShare) && (canShare || isOwner)

  return (
    <Card className={cn('border border-border bg-card p-5 shadow-sm', className)}>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            {session.connection_name ?? 'SSH Session'}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Connected as{' '}
            <span className="font-medium text-foreground">
              {session.user_name ?? session.user_id}
            </span>
            {session.host ? ` · ${session.host}` : ''}
            {session.port ? `:${session.port}` : ''}
          </p>
        </div>
        <dl className="grid grid-cols-2 gap-4 text-sm text-muted-foreground">
          {startedAt && (
            <div>
              <dt className="font-medium text-foreground">Started</dt>
              <dd>{formatDistanceToNow(startedAt, { addSuffix: true })}</dd>
            </div>
          )}
          {lastSeenAt && (
            <div>
              <dt className="font-medium text-foreground">Last activity</dt>
              <dd>{formatDistanceToNow(lastSeenAt, { addSuffix: true })}</dd>
            </div>
          )}
        </dl>
      </div>

      <div className="mt-4 flex flex-col gap-3 text-sm text-muted-foreground">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2 font-medium text-foreground">
            <Users className="h-4 w-4" />
            Participants
          </div>
          {shareButtonVisible && (
            <Button
              variant="outline"
              size="sm"
              onClick={onOpenShare}
              className="flex items-center gap-2"
            >
              <UserPlus className="h-4 w-4" /> Manage
            </Button>
          )}
        </div>

        {participantList.length === 0 ? (
          <div className="rounded-md border border-dashed border-border/70 px-3 py-2 text-xs text-muted-foreground">
            No one else is viewing this session yet.
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            {participantList.slice(0, 5).map((participant) => (
              <Badge
                key={participant.user_id}
                variant={participant.is_write_holder ? 'default' : 'secondary'}
                className={cn('flex items-center gap-1', participant.is_write_holder && 'pl-2')}
              >
                {participant.is_write_holder && <Key className="h-3 w-3" aria-hidden />}
                {participant.user_name ?? participant.user_id}
              </Badge>
            ))}
            {participantList.length > 5 && (
              <Badge variant="outline">+{participantList.length - 5} more</Badge>
            )}
          </div>
        )}
      </div>
    </Card>
  )
}

export default SshWorkspaceHeader
