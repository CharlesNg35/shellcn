import { useMemo } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchConnectionSummary } from '@/lib/api/connections'
import type { ConnectionProtocolSummary } from '@/types/connections'
import { ApiError } from '@/lib/api/http'

export const CONNECTION_SUMMARY_QUERY_KEY = ['connections', 'summary'] as const

type SummaryQueryOptions = Omit<
  UseQueryOptions<ConnectionProtocolSummary[], ApiError>,
  'queryKey' | 'queryFn'
>

export function useConnectionSummary(teamId?: string, options?: SummaryQueryOptions) {
  const queryKey = useMemo(
    () => [...CONNECTION_SUMMARY_QUERY_KEY, teamId ?? 'all'] as const,
    [teamId]
  )

  return useQuery<ConnectionProtocolSummary[], ApiError>({
    queryKey,
    queryFn: () => fetchConnectionSummary(teamId ? { team_id: teamId } : undefined),
    staleTime: 60_000,
    ...options,
  })
}
