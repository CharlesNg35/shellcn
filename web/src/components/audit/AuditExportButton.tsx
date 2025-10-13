import { useState } from 'react'
import { Download } from 'lucide-react'
import type { AuditLogEntry, AuditLogExportParams } from '@/types/audit'
import { Button } from '@/components/ui/Button'
import { auditApi } from '@/lib/api/audit'
import { toast } from '@/lib/utils/toast'

interface AuditExportButtonProps {
  params: AuditLogExportParams
  disabled?: boolean
  filename?: string
  className?: string
}

function toCsvValue(value: unknown): string {
  if (value === null || value === undefined) {
    return ''
  }

  if (typeof value === 'string') {
    const needsEscaping = value.includes('"') || value.includes(',') || value.includes('\n')
    const escaped = value.replace(/"/g, '""')
    return needsEscaping ? `"${escaped}"` : escaped
  }

  try {
    const serialized = JSON.stringify(value)
    return toCsvValue(serialized)
  } catch {
    return String(value)
  }
}

function logsToCsv(logs: AuditLogEntry[]) {
  const headers = [
    'id',
    'timestamp',
    'username',
    'email',
    'action',
    'resource',
    'result',
    'ip_address',
    'user_agent',
    'metadata',
  ]

  const rows = logs.map((log) => {
    const metadataValue =
      log.metadata === undefined || log.metadata === null ? '' : toCsvValue(log.metadata)

    return [
      toCsvValue(log.id),
      toCsvValue(log.created_at),
      toCsvValue(log.username),
      toCsvValue(log.user?.email ?? ''),
      toCsvValue(log.action),
      toCsvValue(log.resource ?? ''),
      toCsvValue(log.result),
      toCsvValue(log.ip_address ?? ''),
      toCsvValue(log.user_agent ?? ''),
      metadataValue,
    ].join(',')
  })

  return [headers.join(','), ...rows].join('\r\n')
}

export function AuditExportButton({
  params,
  disabled,
  filename,
  className,
}: AuditExportButtonProps) {
  const [isExporting, setIsExporting] = useState(false)

  const handleExport = async () => {
    setIsExporting(true)
    try {
      const logs = await auditApi.export(params)
      const csv = logsToCsv(logs)
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
      const objectUrl = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = objectUrl
      link.download = filename ?? `audit-logs-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(objectUrl)

      toast.success('Audit logs exported', {
        description:
          logs.length === 0
            ? 'No audit entries matched your filters.'
            : `${logs.length.toLocaleString()} audit entries exported successfully.`,
      })
    } catch (error) {
      toast.error('Unable to export audit logs', {
        description:
          error instanceof Error ? error.message : 'An unexpected error occurred during export.',
      })
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <Button
      type="button"
      variant="outline"
      className={className}
      onClick={handleExport}
      disabled={disabled || isExporting}
      loading={isExporting}
    >
      <Download className="h-4 w-4" />
      Export CSV
    </Button>
  )
}
