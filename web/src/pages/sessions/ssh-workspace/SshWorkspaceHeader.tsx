import { useMemo } from 'react'
import { Users, UserPlus, Key } from 'lucide-react'

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
    <div
      className={cn(
        'flex flex-wrap items-center justify-between gap-3 border-b border-border/50 pb-3',
        className
      )}
    >
      <div className="flex items-center gap-4">
        <div>
          <h1 className="text-base font-semibold text-foreground">
            {session.connection_name ?? 'SSH Session'}
          </h1>
          <p className="text-xs text-muted-foreground">
            {session.user_name ?? session.user_id}
            {session.host ? ` @ ${session.host}` : ''}
            {session.port && session.port !== 22 ? `:${session.port}` : ''}
          </p>
        </div>

        {participantList.length > 0 && (
          <div className="flex items-center gap-2 border-l border-border/50 pl-4">
            <Users className="h-3.5 w-3.5 text-muted-foreground" aria-hidden />
            <div className="flex flex-wrap items-center gap-1.5">
              {participantList.slice(0, 3).map((participant) => (
                <Badge
                  key={participant.user_id}
                  variant={participant.is_write_holder ? 'default' : 'secondary'}
                  className={cn('h-5 text-xs', participant.is_write_holder && 'gap-1 pl-1.5 pr-2')}
                >
                  {participant.is_write_holder && <Key className="h-2.5 w-2.5" aria-hidden />}
                  {participant.user_name ?? participant.user_id}
                </Badge>
              ))}
              {participantList.length > 3 && (
                <Badge variant="outline" className="h-5 text-xs">
                  +{participantList.length - 3}
                </Badge>
              )}
            </div>
          </div>
        )}
      </div>

      {shareButtonVisible && (
        <Button variant="outline" size="sm" onClick={onOpenShare} className="h-7 gap-1.5 text-xs">
          <UserPlus className="h-3.5 w-3.5" aria-hidden />
          Share
        </Button>
      )}
    </div>
  )
}

export default SshWorkspaceHeader
