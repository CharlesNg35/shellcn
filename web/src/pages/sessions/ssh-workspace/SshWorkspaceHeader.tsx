import { formatDistanceToNow } from 'date-fns'
import { Card } from '@/components/ui/Card'
import type { ActiveConnectionSession } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

interface SshWorkspaceHeaderProps {
  session: ActiveConnectionSession
  className?: string
}

export function SshWorkspaceHeader({ session, className }: SshWorkspaceHeaderProps) {
  const startedAt = session.started_at ? new Date(session.started_at) : undefined
  const lastSeenAt = session.last_seen_at ? new Date(session.last_seen_at) : undefined

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
    </Card>
  )
}

export default SshWorkspaceHeader
