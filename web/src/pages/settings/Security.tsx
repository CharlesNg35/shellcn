import { type ComponentProps, type ComponentType, type ReactNode, useMemo, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Gauge,
  RefreshCw,
  Shield,
  SignalHigh,
  SignalLow,
  XCircle,
} from 'lucide-react'
import { format, formatDistanceToNow } from 'date-fns'
import { PageHeader } from '@/components/layout/PageHeader'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { PERMISSIONS } from '@/constants/permissions'
import { useSecurityAudit } from '@/hooks/useSecurityAudit'
import { useHealthLiveness, useHealthReadiness, useMonitoringSummary } from '@/hooks/useMonitoring'
import type { HealthStatus, MaintenanceJobSummary, ProtocolSummary } from '@/types/monitoring'
import type { SecurityAuditCheck, SecurityCheckStatus } from '@/types/security'

const SECURITY_TABS = [
  { value: 'audit', label: 'Security Audit' },
  { value: 'monitoring', label: 'Monitoring' },
] as const

const STATUS_CONFIG: Record<
  SecurityCheckStatus,
  {
    label: string
    icon: ComponentType<{ className?: string }>
    badgeVariant: ComponentProps<typeof Badge>['variant']
    indicatorClass: string
  }
> = {
  pass: {
    label: 'Pass',
    icon: CheckCircle2,
    badgeVariant: 'success',
    indicatorClass: 'text-emerald-500',
  },
  warn: {
    label: 'Warning',
    icon: AlertTriangle,
    badgeVariant: 'secondary',
    indicatorClass: 'text-amber-500',
  },
  fail: {
    label: 'Failure',
    icon: XCircle,
    badgeVariant: 'destructive',
    indicatorClass: 'text-rose-500',
  },
}

const HEALTH_STATUS_META: Record<
  HealthStatus,
  {
    label: string
    variant: ComponentProps<typeof Badge>['variant']
    icon: ComponentType<{ className?: string }>
  }
> = {
  up: { label: 'Healthy', variant: 'success', icon: CheckCircle2 },
  degraded: { label: 'Degraded', variant: 'secondary', icon: AlertTriangle },
  down: { label: 'Unavailable', variant: 'destructive', icon: XCircle },
}

const durationFormatter = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 2,
})

export function Security() {
  const [activeTab, setActiveTab] = useState<(typeof SECURITY_TABS)[number]['value']>('audit')
  const auditQuery = useSecurityAudit()

  const headerAction =
    activeTab === 'audit' ? (
      <PermissionGuard permission={PERMISSIONS.SECURITY.AUDIT} fallback={null}>
        <Button
          type="button"
          variant="outline"
          onClick={() => auditQuery.refetch()}
          disabled={auditQuery.isFetching}
        >
          <RefreshCw className="mr-1 h-4 w-4" />
          {auditQuery.isFetching ? 'Running audit…' : 'Run audit'}
        </Button>
      </PermissionGuard>
    ) : null

  return (
    <div className="space-y-6">
      <PageHeader
        title="Security & Monitoring"
        description="Review automated security checks and monitor system health to ensure dependable operations."
        action={headerAction}
      />

      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as typeof activeTab)}>
        <TabsList className="flex-wrap gap-2 bg-muted/40 p-1">
          {SECURITY_TABS.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="audit" className="mt-4 space-y-6">
          <SecurityAuditPanel auditQuery={auditQuery} />
        </TabsContent>

        <TabsContent value="monitoring" className="mt-4 space-y-6">
          <PermissionGuard
            permission={PERMISSIONS.MONITORING.VIEW}
            fallback={
              <EmptyState
                icon={Shield}
                title="Monitoring unavailable"
                description="You do not have permission to view monitoring data. Please contact an administrator."
              />
            }
          >
            <MonitoringPanel />
          </PermissionGuard>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function SecurityAuditPanel({ auditQuery }: { auditQuery: ReturnType<typeof useSecurityAudit> }) {
  const summaryCards = useMemo(() => {
    return (['pass', 'warn', 'fail'] as SecurityCheckStatus[]).map((status) => {
      const config = STATUS_CONFIG[status]
      const value = getSummaryValue(auditQuery.data?.summary, status)
      return {
        status,
        label: status === 'pass' ? 'Passing checks' : status === 'warn' ? 'Warnings' : 'Failures',
        value,
        icon: config.icon,
        indicatorClass: config.indicatorClass,
      }
    })
  }, [auditQuery.data?.summary])

  const lastCheckedAt = useMemo(() => {
    const value = auditQuery.data?.checked_at
    if (!value) {
      return null
    }
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
      return null
    }
    return date
  }, [auditQuery.data?.checked_at])

  const checks = auditQuery.data?.checks ?? []

  let content: ReactNode

  if (auditQuery.isLoading) {
    content = renderAuditLoadingState()
  } else if (auditQuery.isError) {
    content = (
      <EmptyState
        icon={Shield}
        title="Unable to load audit results"
        description={
          auditQuery.error?.message ??
          'An unexpected error occurred while loading the security audit.'
        }
        action={
          <Button type="button" onClick={() => auditQuery.refetch()}>
            Retry
          </Button>
        }
      />
    )
  } else {
    content = (
      <div className="space-y-6">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {summaryCards.map((card) => {
            const Icon = card.icon
            return (
              <Card key={card.status}>
                <CardContent className="flex items-center justify-between gap-3 p-5">
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-muted-foreground">{card.label}</p>
                    <p className="text-2xl font-semibold text-foreground">{card.value}</p>
                  </div>
                  <Icon className={`h-8 w-8 ${card.indicatorClass}`} />
                </CardContent>
              </Card>
            )
          })}
        </div>

        <div className="flex flex-col gap-2 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          {lastCheckedAt ? (
            <span>
              Last checked {formatDistanceToNow(lastCheckedAt, { addSuffix: true })} (
              {format(lastCheckedAt, 'PPpp')})
            </span>
          ) : (
            <span>Audit has not been executed yet.</span>
          )}
          {auditQuery.isFetching ? <span>Refreshing audit results…</span> : null}
        </div>

        {renderAuditChecks(checks)}
      </div>
    )
  }

  return (
    <PermissionGuard
      permission={PERMISSIONS.SECURITY.AUDIT}
      fallback={
        <EmptyState
          icon={Shield}
          title="Security audit unavailable"
          description="You do not have permission to view security audit results. Please contact an administrator."
        />
      }
    >
      {content}
    </PermissionGuard>
  )
}

function MonitoringPanel() {
  const summaryQuery = useMonitoringSummary()
  const readinessQuery = useHealthReadiness()
  const livenessQuery = useHealthLiveness()

  if (summaryQuery.isLoading && !summaryQuery.data) {
    return <MonitoringLoadingState />
  }

  if (summaryQuery.isError) {
    if (summaryQuery.error?.status === 404) {
      return (
        <EmptyState
          icon={Gauge}
          title="Monitoring disabled"
          description="Monitoring endpoints are currently disabled. Enable monitoring in configuration to surface system health data."
        />
      )
    }

    return (
      <EmptyState
        icon={Gauge}
        title="Unable to load monitoring data"
        description={
          summaryQuery.error?.message ??
          'An unexpected error occurred while loading monitoring data.'
        }
        action={
          <Button type="button" onClick={() => summaryQuery.refetch()}>
            Retry
          </Button>
        }
      />
    )
  }

  const summaryData = summaryQuery.data

  if (!summaryData) {
    return <MonitoringLoadingState />
  }

  const { summary, prometheus } = summaryData
  const readiness = readinessQuery.data
  const liveness = livenessQuery.data
  const readinessStatus = readiness?.status ?? 'degraded'
  const livenessStatus = liveness?.status ?? 'degraded'

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <StatusCard
          title="Readiness"
          status={readinessStatus}
          checkedAt={readiness?.checked_at}
          isLoading={readinessQuery.isLoading}
          icon={SignalHigh}
        />
        <StatusCard
          title="Liveness"
          status={livenessStatus}
          checkedAt={liveness?.checked_at}
          isLoading={livenessQuery.isLoading}
          icon={Activity}
        />
        <Card>
          <CardContent className="space-y-2 p-5">
            <p className="text-sm font-medium text-muted-foreground">Prometheus</p>
            <div className="flex items-center justify-between">
              <span className="text-lg font-semibold text-foreground">
                {prometheus.enabled ? 'Enabled' : 'Disabled'}
              </span>
              <Gauge className="h-6 w-6 text-muted-foreground" />
            </div>
            {prometheus.enabled ? (
              <p className="text-xs text-muted-foreground">
                Endpoint: <span className="font-mono text-foreground">{prometheus.endpoint}</span>
              </p>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <MetricCard
          title="Authentication"
          icon={CheckCircle2}
          metrics={[
            { label: 'Success', value: summary.auth.success },
            { label: 'Failure', value: summary.auth.failure },
            { label: 'Errors', value: summary.auth.error },
          ]}
        />
        <MetricCard
          title="Permissions"
          icon={Shield}
          metrics={[
            { label: 'Allowed', value: summary.permissions.allowed },
            { label: 'Denied', value: summary.permissions.denied },
            { label: 'Errors', value: summary.permissions.error },
          ]}
        />
        <MetricCard
          title="Sessions"
          icon={SignalLow}
          metrics={[
            { label: 'Active', value: summary.sessions.active },
            { label: 'Completed', value: summary.sessions.completed },
            {
              label: 'Avg duration',
              value: formatSeconds(summary.sessions.average_duration_seconds),
            },
          ]}
        />
      </div>

      <HealthChecksTable checks={readiness?.checks ?? []} isLoading={readinessQuery.isLoading} />

      <MaintenanceTable jobs={summary.maintenance.jobs} />
      <ProtocolTable protocols={summary.protocols} />
    </div>
  )
}

function MetricCard({
  title,
  icon: Icon,
  metrics,
}: {
  title: string
  icon: ComponentType<{ className?: string }>
  metrics: { label: string; value: number | string }[]
}) {
  return (
    <Card>
      <CardContent className="space-y-4 p-5">
        <div className="flex items-center justify-between">
          <p className="text-sm font-medium text-muted-foreground">{title}</p>
          <Icon className="h-5 w-5 text-muted-foreground" />
        </div>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
          {metrics.map((metric) => (
            <div key={metric.label} className="rounded-md bg-muted/40 p-3 text-sm">
              <p className="text-xs text-muted-foreground">{metric.label}</p>
              <p className="text-base font-semibold text-foreground">{metric.value}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function StatusCard({
  title,
  status,
  checkedAt,
  isLoading,
  icon: Icon,
}: {
  title: string
  status: HealthStatus
  checkedAt?: string
  isLoading: boolean
  icon: ComponentType<{ className?: string }>
}) {
  const meta = HEALTH_STATUS_META[status]
  return (
    <Card>
      <CardContent className="space-y-3 p-5">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium text-muted-foreground">{title}</p>
            <div className="mt-2 flex items-center gap-2">
              <Icon className="h-5 w-5 text-muted-foreground" />
              <Badge variant={meta.variant}>{meta.label}</Badge>
            </div>
          </div>
          {isLoading ? <Skeleton className="h-5 w-16" /> : null}
        </div>
        <p className="text-xs text-muted-foreground">
          {checkedAt
            ? `Checked ${formatDistanceToNow(new Date(checkedAt), { addSuffix: true })}`
            : '\u2014'}
        </p>
      </CardContent>
    </Card>
  )
}

function HealthChecksTable({
  checks,
  isLoading,
}: {
  checks: { component: string; status: HealthStatus; details?: string; duration?: number }[]
  isLoading: boolean
}) {
  if (isLoading && !checks.length) {
    return <MonitoringLoadingState />
  }

  if (!checks.length) {
    return (
      <Card>
        <CardContent className="p-6">
          <EmptyState
            icon={Shield}
            title="No readiness checks"
            description="No readiness checks are registered for this environment."
          />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="space-y-4 p-5">
        <div className="flex items-center justify-between">
          <p className="text-sm font-medium text-muted-foreground">Health checks</p>
          {isLoading ? <Skeleton className="h-5 w-24" /> : null}
        </div>
        <div className="space-y-3">
          {checks.map((check) => {
            const meta = HEALTH_STATUS_META[check.status]
            const StatusIcon = meta.icon
            return (
              <div
                key={check.component}
                className="flex flex-col gap-2 rounded-lg border border-border/70 bg-card/50 p-4 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="flex items-center gap-3">
                  <StatusIcon className="h-5 w-5 text-muted-foreground" />
                  <div>
                    <p className="font-medium text-foreground">{check.component}</p>
                    {check.details ? (
                      <p className="text-xs text-muted-foreground">{check.details}</p>
                    ) : null}
                  </div>
                </div>
                <div className="flex items-center gap-3 text-sm">
                  <Badge variant={meta.variant}>{meta.label}</Badge>
                  {check.duration ? (
                    <span className="text-xs text-muted-foreground">
                      {formatDuration(check.duration)}
                    </span>
                  ) : null}
                </div>
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

function MaintenanceTable({ jobs }: { jobs: MaintenanceJobSummary[] }) {
  if (!jobs.length) {
    return (
      <Card>
        <CardContent className="p-6">
          <EmptyState
            icon={RefreshCw}
            title="No maintenance activity"
            description="Maintenance jobs have not produced any results yet."
          />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="space-y-4 p-5">
        <p className="text-sm font-medium text-muted-foreground">Maintenance jobs</p>
        <div className="space-y-3">
          {jobs.map((job) => (
            <div
              key={job.job}
              className="flex flex-col gap-2 rounded-lg border border-border/70 bg-card/40 p-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div>
                <p className="font-semibold text-foreground">{job.job}</p>
                <p className="text-xs text-muted-foreground">
                  Last run:{' '}
                  {job.last_run_at
                    ? formatDistanceToNow(new Date(job.last_run_at), { addSuffix: true })
                    : 'N/A'}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                <Badge variant={job.last_status === 'success' ? 'success' : 'secondary'}>
                  {job.last_status === 'success' ? 'Success' : job.last_status}
                </Badge>
                <span>Duration: {formatDuration(job.last_duration)}</span>
                <span>Failures: {job.consecutive_failures}</span>
                <span>Runs: {job.total_runs}</span>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function ProtocolTable({ protocols }: { protocols: ProtocolSummary[] }) {
  if (!protocols.length) {
    return (
      <Card>
        <CardContent className="p-6">
          <EmptyState
            icon={Shield}
            title="No protocol telemetry"
            description="Protocol launch activity has not been recorded yet."
          />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="space-y-3 p-5">
        <p className="text-sm font-medium text-muted-foreground">Protocol launches</p>
        <div className="space-y-2">
          {protocols.map((protocol) => (
            <div
              key={protocol.protocol}
              className="flex flex-col gap-2 rounded-lg border border-border/70 bg-card/40 p-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div>
                <p className="font-semibold text-foreground">{protocol.protocol}</p>
                <p className="text-xs text-muted-foreground">
                  Last activity:{' '}
                  {protocol.last_completed_at
                    ? formatDistanceToNow(new Date(protocol.last_completed_at), {
                        addSuffix: true,
                      })
                    : 'N/A'}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                <span>Success: {protocol.success}</span>
                <span>Failure: {protocol.failure}</span>
                <span>Avg latency: {formatSeconds(protocol.average_latency_seconds)}</span>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function MonitoringLoadingState() {
  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
      {[0, 1, 2].map((index) => (
        <Card key={index}>
          <CardContent className="space-y-3 p-5">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-6 w-16" />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function renderAuditLoadingState() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {[0, 1, 2].map((index) => (
          <Card key={index}>
            <CardContent className="space-y-3 p-5">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-6 w-16" />
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="space-y-3">
        {[0, 1].map((index) => (
          <div key={index} className="rounded-lg border border-border/70 bg-card/50 p-4">
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="mt-3 h-12 w-full" />
          </div>
        ))}
      </div>
    </div>
  )
}

function renderAuditChecks(items: SecurityAuditCheck[]) {
  if (!items.length) {
    return (
      <EmptyState
        icon={Shield}
        title="No checks available"
        description="Security checks have not produced any results yet."
      />
    )
  }

  return (
    <div className="space-y-3">
      {items.map((check) => {
        const config = STATUS_CONFIG[check.status]
        const Icon = config.icon
        const details = renderDetails(check.details)

        return (
          <div
            key={check.id}
            className="space-y-3 rounded-lg border border-border/70 bg-card/50 p-4 shadow-sm"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="flex items-start gap-3">
                <Icon className={`mt-0.5 h-5 w-5 ${config.indicatorClass}`} />
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold text-foreground">{check.message}</span>
                    <Badge variant={config.badgeVariant}>{config.label}</Badge>
                  </div>
                  <p className="text-xs text-muted-foreground">Check ID: {check.id}</p>
                </div>
              </div>
            </div>

            {check.remediation ? (
              <div className="rounded-md bg-muted/40 p-3 text-sm text-muted-foreground">
                <span className="font-medium text-foreground">Remediation:</span>{' '}
                {check.remediation}
              </div>
            ) : null}

            {details ? (
              <pre className="overflow-x-auto rounded-md bg-muted/30 p-3 text-xs text-muted-foreground">
                {details}
              </pre>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}

function getSummaryValue(
  summary: Record<string, number | undefined> | undefined,
  key: SecurityCheckStatus
) {
  if (!summary) {
    return 0
  }
  return summary[key] ?? 0
}

function renderDetails(details: unknown) {
  if (details === null || details === undefined) {
    return null
  }

  if (typeof details === 'string') {
    return details
  }

  try {
    return JSON.stringify(details, null, 2)
  } catch {
    return String(details)
  }
}

function formatDuration(duration?: number) {
  if (duration === undefined || duration === null) {
    return '—'
  }
  if (duration === 0) {
    return '0s'
  }
  const seconds = duration / 1_000_000_000
  if (seconds < 1) {
    return `${durationFormatter.format(seconds)}s`
  }
  if (seconds < 60) {
    return `${durationFormatter.format(seconds)}s`
  }
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (minutes < 60) {
    return `${minutes}m ${durationFormatter.format(remainingSeconds)}s`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}h ${remainingMinutes}m`
}

function formatSeconds(seconds?: number) {
  if (seconds === undefined || seconds === null) {
    return '—'
  }
  if (seconds < 1) {
    return `${durationFormatter.format(seconds)}s`
  }
  if (seconds < 60) {
    return `${durationFormatter.format(seconds)}s`
  }
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (minutes < 60) {
    return `${minutes}m ${durationFormatter.format(remainingSeconds)}s`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}h ${remainingMinutes}m`
}
