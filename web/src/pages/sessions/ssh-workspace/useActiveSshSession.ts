import { useMemo } from 'react'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import type { ActiveConnectionSession } from '@/types/connections'

interface UseActiveSshSessionResult {
  session?: ActiveConnectionSession
  activeSessions: ActiveConnectionSession[]
  isLoading: boolean
  isError: boolean
}

export function useActiveSshSession(sessionId: string): UseActiveSshSessionResult {
  const query = useActiveConnections({
    protocol_id: 'ssh',
    enabled: Boolean(sessionId),
    refetchInterval: 15_000,
  })

  const session = useMemo(
    () => query.data?.find((record) => record.id === sessionId),
    [query.data, sessionId]
  )

  return {
    session,
    activeSessions: query.data ?? [],
    isLoading: Boolean(query.isLoading),
    isError: Boolean(query.isError),
  }
}
