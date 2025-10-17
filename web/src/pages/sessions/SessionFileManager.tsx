import { useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { SftpWorkspace } from '@/components/workspace/SftpWorkspace'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { formatDistanceToNow } from 'date-fns'
import { sessionSupportsSftp } from '@/lib/utils/sessionCapabilities'

export function SessionFileManager() {
  const { sessionId = '' } = useParams<{ sessionId: string }>()
  const { setOverride, clearOverride } = useBreadcrumb()

  const {
    data: activeSessions,
    isLoading,
    isError,
  } = useActiveConnections({
    protocol_id: 'ssh',
    enabled: Boolean(sessionId),
    refetchInterval: 20_000,
  })

  const session = useMemo(() => {
    return activeSessions?.find((record) => record.id === sessionId)
  }, [activeSessions, sessionId])

  const currentUserQuery = useCurrentUser()
  const currentUser = currentUserQuery.data

  const currentUserId = currentUser?.id
  const currentUserDisplayName = useMemo(() => {
    if (!currentUser) {
      return undefined
    }
    const fullName = [currentUser.first_name, currentUser.last_name]
      .filter(Boolean)
      .join(' ')
      .trim()
    return fullName || currentUser.username || currentUser.email || currentUser.id
  }, [currentUser])

  const canWrite = useMemo(() => {
    if (!session || !currentUserId) {
      return false
    }
    if (session.write_holder && session.write_holder === currentUserId) {
      return true
    }
    if (session.owner_user_id && session.owner_user_id === currentUserId) {
      return true
    }
    if (session.user_id === currentUserId) {
      return true
    }
    const participant = session.participants?.[currentUserId]
    if (participant && participant.access_mode?.toLowerCase() === 'write') {
      return true
    }
    return false
  }, [currentUserId, session])

  useEffect(() => {
    const label = session?.connection_name ?? session?.connection_id
    if (sessionId && label) {
      const path = `/active-sessions/${sessionId}/sftp`
      setOverride(path, `${label} · SFTP`)
      return () => {
        clearOverride(path)
      }
    }
    return undefined
  }, [clearOverride, session?.connection_id, session?.connection_name, sessionId, setOverride])

  if (!sessionId) {
    return (
      <EmptyState
        title="Session not specified"
        description="Provide an active session identifier to open the file manager."
        className="h-full"
      />
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
        Loading session details…
      </div>
    )
  }

  if (isError || !session) {
    return (
      <EmptyState
        title="Session unavailable"
        description="The requested session could not be found or is no longer active."
        className="h-full"
      />
    )
  }

  if (!sessionSupportsSftp(session)) {
    return (
      <EmptyState
        title="SFTP disabled"
        description="File manager is not available for this session."
        className="h-full"
      />
    )
  }

  const startedAt = session.started_at ? new Date(session.started_at) : undefined
  const lastSeenAt = session.last_seen_at ? new Date(session.last_seen_at) : undefined

  return (
    <div className="flex h-full flex-col gap-6">
      <Card className="border border-border bg-card p-5 shadow-sm">
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

      <div className="flex-1 overflow-hidden">
        <SftpWorkspace
          sessionId={sessionId}
          canWrite={canWrite}
          currentUserId={currentUserId}
          currentUserName={currentUserDisplayName}
          participants={session.participants}
        />
      </div>
    </div>
  )
}

export default SessionFileManager
