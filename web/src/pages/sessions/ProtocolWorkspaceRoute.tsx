import { useMemo } from 'react'
import { Navigate, useParams } from 'react-router-dom'
import { Loader2 } from 'lucide-react'

import { EmptyState } from '@/components/ui/EmptyState'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { getWorkspaceDescriptorForProtocol } from '@/workspaces/protocolWorkspaceRegistry'
import type { WorkspaceMountProps } from '@/workspaces/types'

export function ProtocolWorkspaceRoute() {
  const { sessionId: paramSessionId } = useParams<{ sessionId: string }>()
  const sessionId = paramSessionId ?? ''

  const {
    data: activeSessions = [],
    isLoading,
    isError,
  } = useActiveConnections({
    enabled: Boolean(sessionId),
    refetchInterval: 15_000,
  })

  const session = useMemo(
    () => activeSessions.find((record) => record.id === sessionId),
    [activeSessions, sessionId]
  )

  if (!sessionId) {
    return <Navigate to="/dashboard" replace />
  }

  if (isLoading && !session) {
    return (
      <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" />
        Preparing workspace…
      </div>
    )
  }

  if (!session && !isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <EmptyState
          title="Session not found"
          description="The requested session is no longer active or may have been closed."
        />
      </div>
    )
  }

  const descriptor = getWorkspaceDescriptorForProtocol(session?.protocol_id)
  const MountComponent = descriptor.mount

  const mountProps: WorkspaceMountProps = {
    sessionId,
    session: session,
    allSessions: activeSessions,
    descriptor,
    isLoading,
    isError,
  }

  return <MountComponent {...mountProps} />
}

export default ProtocolWorkspaceRoute
