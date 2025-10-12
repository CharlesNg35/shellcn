import type { ComponentProps } from 'react'
import { format, formatDistanceToNow } from 'date-fns'
import { ShieldOff } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { EmptyState } from '@/components/ui/EmptyState'
import { Skeleton } from '@/components/ui/Skeleton'
import { useProfileSessions } from '@/hooks/useProfileSettings'
import type { SessionStatus } from '@/types/sessions'

const STATUS_LABELS: Record<SessionStatus, string> = {
  active: 'Active',
  revoked: 'Revoked',
  expired: 'Expired',
}

const STATUS_VARIANTS: Record<SessionStatus, ComponentProps<typeof Badge>['variant']> = {
  active: 'success',
  revoked: 'destructive',
  expired: 'outline',
}

function formatRelative(value: string | null | undefined): string {
  if (!value) {
    return 'Unknown'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }
  return formatDistanceToNow(date, { addSuffix: true })
}

function formatAbsolute(value: string | null | undefined): string {
  if (!value) {
    return 'Unknown'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }
  return format(date, 'PPpp')
}

export function ProfileSessionsPanel() {
  const { sessions, stats, query, revokeSession, revokeOtherSessions } = useProfileSessions()

  const revokingSessionId =
    revokeSession.isPending && typeof revokeSession.variables === 'string'
      ? revokeSession.variables
      : undefined

  const revokeOthersDisabled = revokeOtherSessions.isPending || stats.otherActive === 0

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <CardTitle>Active Sessions</CardTitle>
          <CardDescription>
            Review devices that are signed in with your account and revoke access you no longer
            recognise.
          </CardDescription>
        </div>
        <Button
          type="button"
          variant="destructive"
          onClick={() => revokeOtherSessions.mutate()}
          disabled={revokeOthersDisabled}
          loading={revokeOtherSessions.isPending}
        >
          <ShieldOff className="mr-2 h-4 w-4" />
          Revoke Other Sessions
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
          <div>
            <p className="text-xs uppercase text-muted-foreground">Total</p>
            <p className="text-lg font-semibold text-foreground">{stats.total}</p>
          </div>
          <div>
            <p className="text-xs uppercase text-muted-foreground">Active</p>
            <p className="text-lg font-semibold text-foreground">{stats.active}</p>
          </div>
          <div>
            <p className="text-xs uppercase text-muted-foreground">Revoked</p>
            <p className="text-lg font-semibold text-foreground">{stats.revoked}</p>
          </div>
          <div>
            <p className="text-xs uppercase text-muted-foreground">Expired</p>
            <p className="text-lg font-semibold text-foreground">{stats.expired}</p>
          </div>
        </div>

        {query.isError ? (
          <EmptyState
            icon={ShieldOff}
            title="Unable to load sessions"
            description={query.error?.message ?? 'Please try again later.'}
            action={
              <Button type="button" size="sm" onClick={() => query.refetch()}>
                Retry
              </Button>
            }
          />
        ) : query.isLoading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((index) => (
              <div
                key={index}
                className="flex flex-col gap-3 rounded-lg border border-border/60 bg-muted/20 p-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="space-y-2">
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="h-3 w-32" />
                </div>
                <div className="flex items-center gap-3">
                  <Skeleton className="h-6 w-20" />
                  <Skeleton className="h-8 w-24" />
                </div>
              </div>
            ))}
          </div>
        ) : sessions.length === 0 ? (
          <EmptyState
            title="No sessions yet"
            description="Sign in from another device to see active sessions displayed here."
          />
        ) : (
          <div className="space-y-2">
            {sessions.map((session) => {
              const isCurrent = session.is_current
              const isActive = session.status === 'active'

              return (
                <div
                  key={session.id}
                  className="flex flex-col gap-3 rounded-lg border border-border/60 bg-muted/10 p-3 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium text-foreground">
                        Signed in {formatRelative(session.created_at)}
                      </span>
                      {isCurrent ? (
                        <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                          Current
                        </Badge>
                      ) : null}
                      <Badge variant={STATUS_VARIANTS[session.status]}>
                        {STATUS_LABELS[session.status]}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      <span className="font-medium text-foreground">Last active:</span>{' '}
                      {formatRelative(session.last_used_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      <span className="font-medium text-foreground">IP:</span>{' '}
                      {session.ip_address || 'Unknown'}
                    </div>
                    {session.user_agent ? (
                      <div className="text-xs text-muted-foreground">
                        <span className="font-medium text-foreground">User agent:</span>{' '}
                        {session.user_agent}
                      </div>
                    ) : null}
                    <div className="text-xs text-muted-foreground">
                      <span className="font-medium text-foreground">Expires:</span>{' '}
                      {formatAbsolute(session.expires_at)}
                    </div>
                  </div>
                  <div className="flex items-center justify-end gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => revokeSession.mutate(session.id)}
                      disabled={!isActive || isCurrent || revokeSession.isPending}
                      loading={revokingSessionId === session.id}
                    >
                      Revoke
                    </Button>
                  </div>
                </div>
              )
            })}
            {query.isFetching ? (
              <p className="text-right text-xs text-muted-foreground">Refreshing sessions…</p>
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
