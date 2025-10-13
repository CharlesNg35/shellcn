import { useQuery } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { monitoringApi } from '@/lib/api/monitoring'
import type { HealthReport, MonitoringSummaryResponse } from '@/types/monitoring'

const SUMMARY_QUERY_KEY = ['monitoring', 'summary'] as const
const READINESS_QUERY_KEY = ['monitoring', 'health', 'readiness'] as const
const LIVENESS_QUERY_KEY = ['monitoring', 'health', 'liveness'] as const

export function useMonitoringSummary() {
  return useQuery<MonitoringSummaryResponse, ApiError>({
    queryKey: SUMMARY_QUERY_KEY,
    queryFn: monitoringApi.fetchSummary,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
    staleTime: 30_000,
  })
}

export function useHealthReadiness() {
  return useQuery<HealthReport, ApiError>({
    queryKey: READINESS_QUERY_KEY,
    queryFn: monitoringApi.fetchReadiness,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
    staleTime: 30_000,
  })
}

export function useHealthLiveness() {
  return useQuery<HealthReport, ApiError>({
    queryKey: LIVENESS_QUERY_KEY,
    queryFn: monitoringApi.fetchLiveness,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
    staleTime: 30_000,
  })
}

export const monitoringQueries = {
  SUMMARY_QUERY_KEY,
  READINESS_QUERY_KEY,
  LIVENESS_QUERY_KEY,
}
