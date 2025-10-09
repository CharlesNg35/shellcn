import { useMemo } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import {
  fetchConnections,
  type ConnectionListResult,
  type FetchConnectionsParams,
} from '@/lib/api/connections'
import { ApiError } from '@/lib/api/http'

export const CONNECTIONS_QUERY_BASE_KEY = ['connections', 'list'] as const

export function getConnectionsQueryKey(params?: FetchConnectionsParams) {
  return [...CONNECTIONS_QUERY_BASE_KEY, params ?? {}] as const
}

type QueryOptions = UseQueryOptions<ConnectionListResult, ApiError>

export function useConnections(params?: FetchConnectionsParams, options?: QueryOptions) {
  const queryKey = useMemo(() => getConnectionsQueryKey(params), [params])

  return useQuery<ConnectionListResult, ApiError>({
    queryKey,
    queryFn: () => fetchConnections(params),
    staleTime: 60_000,
    ...options,
  })
}
