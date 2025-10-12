import { useEffect, useMemo, useState } from 'react'
import { FileText, RefreshCw } from 'lucide-react'
import { AuditFilters, type AuditFilterState } from '@/components/audit/AuditFilters'
import { AuditLogDetailModal } from '@/components/audit/AuditLogDetailModal'
import { AuditExportButton } from '@/components/audit/AuditExportButton'
import { AuditLogTable } from '@/components/audit/AuditLogTable'
import { PageHeader } from '@/components/layout/PageHeader'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { PERMISSIONS } from '@/constants/permissions'
import { useAuditLogs } from '@/hooks/useAuditLogs'
import type { AuditLogEntry, AuditLogExportParams, AuditLogListParams } from '@/types/audit'

const DEFAULT_PAGE_SIZE = 50
const INITIAL_FILTERS: AuditFilterState = {
  result: 'all',
}

function toRFC3339Start(date: string | undefined) {
  if (!date) {
    return undefined
  }
  const [year, month, day] = date.split('-').map((value) => Number.parseInt(value, 10))
  if (
    Number.isNaN(year) ||
    Number.isNaN(month) ||
    Number.isNaN(day) ||
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31
  ) {
    return undefined
  }
  return new Date(Date.UTC(year, month - 1, day, 0, 0, 0, 0)).toISOString()
}

function toRFC3339End(date: string | undefined) {
  if (!date) {
    return undefined
  }
  const [year, month, day] = date.split('-').map((value) => Number.parseInt(value, 10))
  if (
    Number.isNaN(year) ||
    Number.isNaN(month) ||
    Number.isNaN(day) ||
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31
  ) {
    return undefined
  }
  return new Date(Date.UTC(year, month - 1, day, 23, 59, 59, 999)).toISOString()
}

function metadataToString(metadata: unknown) {
  if (metadata === null || metadata === undefined) {
    return ''
  }
  if (typeof metadata === 'string') {
    return metadata
  }
  try {
    return JSON.stringify(metadata)
  } catch {
    return String(metadata)
  }
}

function filterLogs(
  logs: AuditLogEntry[],
  searchTerm?: string,
  actorTerm?: string
): AuditLogEntry[] {
  const normalizedSearch = searchTerm?.trim().toLowerCase()
  const normalizedActor = actorTerm?.trim().toLowerCase()

  return logs.filter((log) => {
    const metadataText = metadataToString(log.metadata).toLowerCase()
    const searchableFields = [
      log.username,
      log.action,
      log.resource ?? '',
      log.ip_address ?? '',
      log.user_agent ?? '',
      metadataText,
    ]
    const actorFields = [log.username, log.user?.email ?? '']

    const matchesSearch =
      !normalizedSearch ||
      searchableFields.some((field) => field.toLowerCase().includes(normalizedSearch))

    const matchesActor =
      !normalizedActor || actorFields.some((field) => field.toLowerCase().includes(normalizedActor))

    return matchesSearch && matchesActor
  })
}

export function AuditLogs() {
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<AuditFilterState>(INITIAL_FILTERS)
  const [selectedLog, setSelectedLog] = useState<AuditLogEntry | null>(null)

  const { action, resource, result, from, to, actor, search } = filters

  useEffect(() => {
    setPage(1)
  }, [action, resource, result, from, to])

  const queryParams = useMemo<AuditLogListParams>(() => {
    const params: AuditLogListParams = {
      page,
      per_page: DEFAULT_PAGE_SIZE,
    }

    if (action) {
      params.action = action
    }
    if (resource) {
      params.resource = resource
    }
    if (result && result !== 'all') {
      params.result = result
    }
    const since = toRFC3339Start(from)
    const until = toRFC3339End(to)
    if (since) {
      params.since = since
    }
    if (until) {
      params.until = until
    }
    if (actor) {
      params.actor = actor
    }
    return params
  }, [page, action, resource, result, from, to, actor])

  const exportParams = useMemo<AuditLogExportParams>(() => {
    const params: AuditLogExportParams = {}
    if (action) {
      params.action = action
    }
    if (resource) {
      params.resource = resource
    }
    if (result && result !== 'all') {
      params.result = result
    }
    const since = toRFC3339Start(from)
    const until = toRFC3339End(to)
    if (since) {
      params.since = since
    }
    if (until) {
      params.until = until
    }
    if (actor) {
      params.actor = actor
    }
    return params
  }, [action, resource, result, from, to, actor])

  const auditQuery = useAuditLogs(queryParams)

  const filteredLogs = useMemo(() => {
    const currentLogs = auditQuery.data?.data ?? []
    return filterLogs(currentLogs, search, actor)
  }, [auditQuery.data?.data, search, actor])

  const handleFilterChange = (next: AuditFilterState) => {
    setFilters(next)
  }

  const handlePageChange = (nextPage: number) => {
    if (nextPage < 1) {
      return
    }
    const totalPages = auditQuery.data?.meta?.total_pages
    if (totalPages && nextPage > totalPages) {
      return
    }
    setPage(nextPage)
  }

  const renderErrorState = () => (
    <EmptyState
      icon={FileText}
      title="Unable to load audit logs"
      description={
        auditQuery.error?.message ?? 'An unexpected error occurred while retrieving audit activity.'
      }
      action={
        <Button type="button" onClick={() => auditQuery.refetch()}>
          Retry
        </Button>
      }
    />
  )

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit Logs"
        description="Inspect system activity, permission changes, and security-sensitive events. Use filters to focus on specific actions, actors, or time ranges."
        action={
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => auditQuery.refetch()}
              disabled={auditQuery.isFetching}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              {auditQuery.isFetching ? 'Refreshing…' : 'Refresh'}
            </Button>
            <PermissionGuard permission={PERMISSIONS.AUDIT.EXPORT}>
              <AuditExportButton params={exportParams} disabled={auditQuery.isLoading} />
            </PermissionGuard>
          </div>
        }
      />

      <PermissionGuard
        permission={PERMISSIONS.AUDIT.VIEW}
        fallback={
          <EmptyState
            icon={FileText}
            title="Audit logs unavailable"
            description="You do not have permission to view audit events. Contact an administrator to request access."
          />
        }
      >
        <div className="space-y-6">
          <AuditFilters filters={filters} onChange={handleFilterChange} />

          {auditQuery.isError ? (
            renderErrorState()
          ) : (
            <AuditLogTable
              logs={filteredLogs}
              meta={auditQuery.data?.meta}
              page={page}
              perPage={DEFAULT_PAGE_SIZE}
              isLoading={auditQuery.isLoading}
              isFetching={auditQuery.isFetching}
              onPageChange={handlePageChange}
              onSelectLog={(log) => setSelectedLog(log)}
            />
          )}
        </div>
      </PermissionGuard>

      <AuditLogDetailModal
        open={selectedLog !== null}
        log={selectedLog}
        onClose={() => setSelectedLog(null)}
      />
    </div>
  )
}
