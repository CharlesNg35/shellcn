import { useMemo } from 'react'
import { format } from 'date-fns'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import type { AuditLogEntry } from '@/types/audit'

interface AuditLogDetailModalProps {
  open: boolean
  log: AuditLogEntry | null
  onClose: () => void
}

function getResultVariant(result: string | undefined) {
  switch ((result ?? '').toLowerCase()) {
    case 'success':
      return 'success'
    case 'failure':
    case 'denied':
    case 'error':
      return 'destructive'
    default:
      return 'secondary'
  }
}

function getResultLabel(result: string | undefined) {
  if (!result) {
    return 'Unknown'
  }
  return result.charAt(0).toUpperCase() + result.slice(1)
}

function formatDate(isoString: string | undefined) {
  if (!isoString) {
    return ''
  }
  const date = new Date(isoString)
  if (Number.isNaN(date.getTime())) {
    return isoString
  }
  try {
    return format(date, 'PPpp')
  } catch {
    return date.toLocaleString()
  }
}

function serializeMetadata(metadata: unknown) {
  if (metadata === null || metadata === undefined) {
    return null
  }

  if (typeof metadata === 'string') {
    const trimmed = metadata.trim()
    if (!trimmed) {
      return null
    }
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return trimmed
    }
  }

  try {
    return JSON.stringify(metadata, null, 2)
  } catch {
    return String(metadata)
  }
}

export function AuditLogDetailModal({ open, log, onClose }: AuditLogDetailModalProps) {
  const metadata = useMemo(() => serializeMetadata(log?.metadata), [log?.metadata])

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="lg"
      title="Audit event details"
      description={
        log
          ? `Captured on ${formatDate(log.created_at)}`
          : 'Review the complete data associated with the selected audit entry.'
      }
    >
      {log ? (
        <div className="space-y-6">
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <Badge variant={getResultVariant(log.result)}>{getResultLabel(log.result)}</Badge>
            <span className="text-muted-foreground">{formatDate(log.created_at)}</span>
            <span className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
              Event ID: {log.id}
            </span>
          </div>

          <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">Actor</dt>
              <dd className="mt-1">
                <p className="font-medium text-foreground">{log.username}</p>
                {log.user?.email ? (
                  <p className="text-xs text-muted-foreground">{log.user.email}</p>
                ) : null}
                {log.user_id ? (
                  <p className="text-xs text-muted-foreground">User ID: {log.user_id}</p>
                ) : null}
              </dd>
            </div>

            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">Action</dt>
              <dd className="mt-1 text-foreground">{log.action}</dd>
            </div>

            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">Resource</dt>
              <dd className="mt-1 text-foreground">{log.resource ?? '—'}</dd>
            </div>

            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">IP address</dt>
              <dd className="mt-1 text-foreground">{log.ip_address ?? '—'}</dd>
            </div>

            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">User agent</dt>
              <dd className="mt-1 text-foreground break-words">{log.user_agent ?? '—'}</dd>
            </div>

            <div>
              <dt className="text-xs font-semibold uppercase text-muted-foreground">
                Last updated
              </dt>
              <dd className="mt-1 text-foreground">{formatDate(log.updated_at)}</dd>
            </div>
          </dl>

          <div className="space-y-2">
            <h4 className="text-xs font-semibold uppercase text-muted-foreground">Metadata</h4>
            {metadata ? (
              <pre className="max-h-64 overflow-auto rounded-lg bg-muted/40 p-4 text-xs text-muted-foreground">
                {metadata}
              </pre>
            ) : (
              <p className="text-sm text-muted-foreground">No metadata recorded for this event.</p>
            )}
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">
          Select an audit log entry to view detailed information including metadata, actor, and
          resource context.
        </p>
      )}
    </Modal>
  )
}
