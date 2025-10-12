import { useMemo } from 'react'
import { useQuery, type UseQueryOptions, type UseQueryResult } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { fetchAuditLogs } from '@/lib/api/audit'
import type { AuditLogListParams, AuditLogListResult } from '@/types/audit'

export const AUDIT_LOGS_QUERY_KEY = ['audit', 'logs'] as const

type QueryParams = Omit<AuditLogListParams, 'search'>

export function getAuditLogsQueryKey(params?: QueryParams) {
  return [...AUDIT_LOGS_QUERY_KEY, params ?? {}] as const
}

type AuditLogsQueryOptions = Omit<
  UseQueryOptions<AuditLogListResult, ApiError, AuditLogListResult, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

export function useAuditLogs(
  params?: AuditLogListParams,
  options?: AuditLogsQueryOptions
): UseQueryResult<AuditLogListResult, ApiError> {
  const queryParams = useMemo<QueryParams | undefined>(() => {
    if (!params) {
      return undefined
    }
    const { search: _search, ...rest } = params
    void _search
    return rest
  }, [params])

  const queryKey = useMemo(() => getAuditLogsQueryKey(queryParams), [queryParams])

  return useQuery<AuditLogListResult, ApiError, AuditLogListResult, readonly unknown[]>({
    queryKey,
    queryFn: () => fetchAuditLogs(queryParams ?? {}),
    placeholderData: (previous) => previous ?? undefined,
    staleTime: 30_000,
    ...(options ?? {}),
  })
}
