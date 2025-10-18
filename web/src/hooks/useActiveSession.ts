import { useMemo } from 'react'

import {
  useActiveConnections,
  type UseActiveConnectionsOptions,
} from '@/hooks/useActiveConnections'
import type { ActiveConnectionSession } from '@/types/connections'

export interface UseActiveSessionOptions {
  protocolId?: string
  teamId?: string
  scope?: UseActiveConnectionsOptions['scope']
  enabled?: boolean
  refetchInterval?: number | false
}

export interface UseActiveSessionResult {
  session?: ActiveConnectionSession
  sessions: ActiveConnectionSession[]
  isLoading: boolean
  isError: boolean
}

export function useActiveSession(
  sessionId: string,
  options: UseActiveSessionOptions = {}
): UseActiveSessionResult {
  const {
    protocolId,
    teamId,
    scope,
    enabled = Boolean(sessionId),
    refetchInterval = 15_000,
  } = options

  const query = useActiveConnections(
    {
      protocol_id: protocolId,
      team_id: teamId,
      scope,
      enabled,
      refetchInterval,
    },
    {
      staleTime: 10_000,
    }
  )

  const session = useMemo(
    () => query.data?.find((record) => record.id === sessionId),
    [query.data, sessionId]
  )

  return {
    session,
    sessions: query.data ?? [],
    isLoading: Boolean(query.isLoading),
    isError: Boolean(query.isError),
  }
}
