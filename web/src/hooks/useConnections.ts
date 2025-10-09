import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchConnections } from '@/lib/api/connections'
import type { ConnectionRecord } from '@/types/connections'
import { ApiError } from '@/lib/api/http'

const CONNECTIONS_QUERY_KEY = ['connections', 'list'] as const

export function useConnections(options?: UseQueryOptions<ConnectionRecord[], ApiError>) {
  return useQuery<ConnectionRecord[], ApiError>({
    queryKey: CONNECTIONS_QUERY_KEY,
    queryFn: fetchConnections,
    staleTime: 60_000,
    ...options,
  })
}
