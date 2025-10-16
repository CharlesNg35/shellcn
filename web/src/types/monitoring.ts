export type HealthStatus = 'up' | 'down' | 'degraded'

export interface MonitoringSummaryResponse {
  summary: MonitoringSummary
  prometheus: PrometheusMetadata
}

export interface MonitoringSummary {
  generated_at: string
  auth: MetricBreakdown
  permissions: PermissionBreakdown
  sessions: SessionSummary
  realtime: RealtimeSummary
  maintenance: MaintenanceSummary
  protocols: ProtocolSummary[]
  web_vitals: WebVitalSummary[]
}

export interface MetricBreakdown {
  success: number
  failure: number
  error: number
}

export interface PermissionBreakdown {
  allowed: number
  denied: number
  error: number
}

export interface SessionSummary {
  active: number
  completed: number
  average_duration_seconds: number
  last_duration: number
  last_ended_at: string
}

export interface FailureRecord {
  stream: string
  type: string
  message: string
  occurred_at: string
}

export interface RealtimeSummary {
  active_connections: number
  broadcasts: number
  failures: number
  last_failure?: FailureRecord | null
}

export interface MaintenanceSummary {
  jobs: MaintenanceJobSummary[]
}

export interface MaintenanceJobSummary {
  job: string
  last_status: string
  last_run_at: string
  last_duration: number
  last_error?: string
  consecutive_failures: number
  consecutive_success: number
  last_success_at: string
  total_runs: number
}

export interface ProtocolSummary {
  protocol: string
  success: number
  failure: number
  last_status: string
  last_duration: number
  last_completed_at: string
  last_error?: string
  average_latency_seconds: number
}

export interface WebVitalSummary {
  metric: string
  last_value: number
  average_value: number
  samples: number
  last_recorded_at: string
  last_rating: string
}

export interface PrometheusMetadata {
  enabled: boolean
  endpoint: string
}

export interface HealthReport {
  success: boolean
  status: HealthStatus
  checks?: HealthCheck[]
  checked_at: string
}

export interface HealthCheck {
  component: string
  status: HealthStatus
  details?: string
  duration?: number
}
