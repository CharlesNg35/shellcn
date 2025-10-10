import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchAvailableProtocols, fetchProtocols } from '@/lib/api/protocols'
import type { ProtocolListResult } from '@/types/protocols'
import { ApiError } from '@/lib/api/http'

const PROTOCOLS_QUERY_KEY = ['protocols', 'catalog'] as const
const AVAILABLE_PROTOCOLS_QUERY_KEY = ['protocols', 'available'] as const

export function useProtocols(options?: UseQueryOptions<ProtocolListResult, ApiError>) {
  return useQuery<ProtocolListResult, ApiError>({
    queryKey: PROTOCOLS_QUERY_KEY,
    queryFn: fetchProtocols,
    ...options,
  })
}

type ProtocolQueryOptions = Omit<
  UseQueryOptions<ProtocolListResult, ApiError>,
  'queryKey' | 'queryFn'
>

export function useAvailableProtocols(options?: ProtocolQueryOptions) {
  return useQuery<ProtocolListResult, ApiError>({
    queryKey: AVAILABLE_PROTOCOLS_QUERY_KEY,
    queryFn: fetchAvailableProtocols,
    ...options,
  })
}
