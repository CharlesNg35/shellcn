import type { ApiResponse } from '@/types/api'
import type { HealthReport, MonitoringSummaryResponse } from '@/types/monitoring'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const MONITORING_SUMMARY_ENDPOINT = '/monitoring/summary'
const HEALTH_READY_ENDPOINT = '/health/ready'
const HEALTH_LIVE_ENDPOINT = '/health/live'
const MONITORING_WEB_VITALS_ENDPOINT = '/monitoring/vitals'

export interface WebVitalMetricPayload {
  metric: string
  value: number
  rating?: string
  navigation_type?: string
  delta?: number
}

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

export async function submitWebVitals(metrics: WebVitalMetricPayload[]): Promise<void> {
  if (!metrics.length) {
    return
  }
  await apiClient.post(
    MONITORING_WEB_VITALS_ENDPOINT,
    { metrics },
    {
      headers: {
        'Content-Type': 'application/json',
      },
    }
  )
}

export const monitoringApi = {
  fetchSummary: fetchMonitoringSummary,
  fetchReadiness: fetchHealthReadiness,
  fetchLiveness: fetchHealthLiveness,
  submitWebVitals,
}
