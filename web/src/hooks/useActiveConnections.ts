import { useMemo } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'

import {
  fetchActiveConnectionSessions,
  type FetchActiveConnectionSessionsParams,
} from '@/lib/api/connections'
import { ApiError } from '@/lib/api/http'
import type { ActiveConnectionSession } from '@/types/connections'

export const ACTIVE_CONNECTIONS_QUERY_KEY = ['connections', 'active'] as const

export interface UseActiveConnectionsOptions extends FetchActiveConnectionSessionsParams {
  protocolIds?: string[]
  enabled?: boolean
  refetchInterval?: number | false
}

type QueryOptions = Omit<
  UseQueryOptions<ActiveConnectionSession[], ApiError>,
  'queryKey' | 'queryFn' | 'enabled' | 'refetchInterval'
>

export function useActiveConnections(
  options: UseActiveConnectionsOptions = {},
  queryOptions?: QueryOptions
) {
  const {
    protocol_id,
    protocolIds,
    team_id,
    scope,
    enabled = true,
    refetchInterval = 20_000,
  } = options

  const normalisedProtocolIds = useMemo(
    () => (protocolIds && protocolIds.length ? [...protocolIds].sort() : undefined),
    [protocolIds]
  )

  const queryKey = useMemo(
    () =>
      [
        ...ACTIVE_CONNECTIONS_QUERY_KEY,
        { protocol_id, protocolIds: normalisedProtocolIds, team_id, scope },
      ] as const,
    [protocol_id, normalisedProtocolIds, team_id, scope]
  )

  return useQuery<ActiveConnectionSession[], ApiError>({
    queryKey,
    queryFn: async () => {
      const sessions = await fetchActiveConnectionSessions({ protocol_id, team_id, scope })
      if (!normalisedProtocolIds?.length) {
        return sessions
      }
      const allowed = new Set(normalisedProtocolIds.map((value) => value.toLowerCase()))
      return sessions.filter((session) =>
        session.protocol_id ? allowed.has(session.protocol_id.toLowerCase()) : false
      )
    },
    staleTime: 15_000,
    refetchInterval,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled,
    ...queryOptions,
  })
}
