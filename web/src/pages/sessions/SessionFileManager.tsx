import { useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import { EmptyState } from '@/components/ui/EmptyState'
import { SftpWorkspace } from '@/components/workspace/SftpWorkspace'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { sessionSupportsSftp } from '@/lib/utils/sessionCapabilities'

export function SessionFileManager() {
  const { sessionId = '' } = useParams<{ sessionId: string }>()
  const { setOverride, clearOverride } = useBreadcrumb()

  const {
    data: activeSessions,
    isLoading,
    isError,
  } = useActiveConnections({
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
      const path = `/active-sessions/${sessionId}`
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

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center justify-between border-b border-border/50 pb-3">
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
      </div>

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
