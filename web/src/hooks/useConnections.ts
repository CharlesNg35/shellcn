import { useMemo } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import {
  fetchConnections,
  type ConnectionListResult,
  type FetchConnectionsParams,
} from '@/lib/api/connections'
import { ApiError } from '@/lib/api/http'

type QueryOptions = UseQueryOptions<ConnectionListResult, ApiError>

export function useConnections(params?: FetchConnectionsParams, options?: QueryOptions) {
  const queryKey = useMemo(() => ['connections', 'list', params ?? {}], [params])

  return useQuery<ConnectionListResult, ApiError>({
    queryKey,
    queryFn: () => fetchConnections(params),
    staleTime: 60_000,
    ...options,
  })
}
