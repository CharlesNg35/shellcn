import { type ComponentProps, type ComponentType, useMemo } from 'react'
import { AlertTriangle, CheckCircle2, RefreshCw, Shield, XCircle } from 'lucide-react'
import { format, formatDistanceToNow } from 'date-fns'
import { PageHeader } from '@/components/layout/PageHeader'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Skeleton } from '@/components/ui/Skeleton'
import { PERMISSIONS } from '@/constants/permissions'
import { useSecurityAudit } from '@/hooks/useSecurityAudit'
import type { SecurityAuditCheck, SecurityCheckStatus } from '@/types/security'

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

export function Security() {
  const auditQuery = useSecurityAudit()

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

  const renderChecks = (items: SecurityAuditCheck[]) => {
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

  const renderLoadingState = () => (
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

  const renderContent = () => {
    if (auditQuery.isLoading) {
      return renderLoadingState()
    }

    if (auditQuery.isError) {
      return (
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
    }

    return (
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

        {renderChecks(checks)}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Security Posture"
        description="Review automated checks that verify critical security controls such as root access, encryption keys, and token configuration."
        action={
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
        }
      />

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
        {renderContent()}
      </PermissionGuard>
    </div>
  )
}
