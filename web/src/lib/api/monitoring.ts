import type { ApiResponse } from '@/types/api'
import type { HealthReport, MonitoringSummaryResponse } from '@/types/monitoring'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const MONITORING_SUMMARY_ENDPOINT = '/monitoring/summary'
const HEALTH_READY_ENDPOINT = '/health/ready'
const HEALTH_LIVE_ENDPOINT = '/health/live'

export async function fetchMonitoringSummary(): Promise<MonitoringSummaryResponse> {
  const response = await apiClient.get<ApiResponse<MonitoringSummaryResponse>>(
    MONITORING_SUMMARY_ENDPOINT
  )
  return unwrapResponse(response)
}

export async function fetchHealthReadiness(): Promise<HealthReport> {
  const response = await apiClient.get<HealthReport>(HEALTH_READY_ENDPOINT)
  return response.data
}

export async function fetchHealthLiveness(): Promise<HealthReport> {
  const response = await apiClient.get<HealthReport>(HEALTH_LIVE_ENDPOINT)
  return response.data
}

export const monitoringApi = {
  fetchSummary: fetchMonitoringSummary,
  fetchReadiness: fetchHealthReadiness,
  fetchLiveness: fetchHealthLiveness,
}
