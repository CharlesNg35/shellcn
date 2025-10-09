import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchAvailableProtocols, fetchProtocols } from '@/lib/api/protocols'
import type { Protocol } from '@/types/protocols'
import { ApiError } from '@/lib/api/http'

const PROTOCOLS_QUERY_KEY = ['protocols', 'catalog'] as const
const AVAILABLE_PROTOCOLS_QUERY_KEY = ['protocols', 'available'] as const

export function useProtocols(options?: UseQueryOptions<Protocol[], ApiError>) {
  return useQuery<Protocol[], ApiError>({
    queryKey: PROTOCOLS_QUERY_KEY,
    queryFn: fetchProtocols,
    ...options,
  })
}

export function useAvailableProtocols(options?: UseQueryOptions<Protocol[], ApiError>) {
  return useQuery<Protocol[], ApiError>({
    queryKey: AVAILABLE_PROTOCOLS_QUERY_KEY,
    queryFn: fetchAvailableProtocols,
    ...options,
  })
}
