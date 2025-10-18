import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { formatDistanceToNow } from 'date-fns'
import { Download, Loader2, RefreshCw } from 'lucide-react'
import { ungzip } from 'pako'

import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { formatBytes } from '@/components/file-manager/utils'
import { downloadSessionRecording } from '@/lib/api/session-recordings'
import { RecordingPlayer } from '@/components/workspace/ssh/RecordingPlayer'
import type { SessionRecordingStatus } from '@/types/session-recording'

interface SessionRecordingDialogProps {
  open: boolean
  onClose: () => void
  sessionId: string
  status?: SessionRecordingStatus
  isLoading: boolean
  onRefresh?: () => void
}

function formatDuration(seconds?: number): string {
  if (!seconds || seconds <= 0) {
    return '—'
  }
  const totalSeconds = Math.floor(seconds)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const remainingSeconds = totalSeconds % 60

  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  if (minutes > 0) {
    return `${minutes}m ${remainingSeconds}s`
  }
  return `${remainingSeconds}s`
}

export function SessionRecordingDialog({
  open,
  onClose,
  sessionId,
  status,
  isLoading,
  onRefresh,
}: SessionRecordingDialogProps) {
  const [recordingBlob, setRecordingBlob] = useState<Blob | null>(null)
  const [castData, setCastData] = useState<string | null>(null)
  const [playerLoading, setPlayerLoading] = useState(false)
  const [playerError, setPlayerError] = useState<string | null>(null)
  const [downloading, setDownloading] = useState(false)

  const recordId = status?.record?.record_id

  useEffect(() => {
    if (!open) {
      setCastData(null)
      setRecordingBlob(null)
      setPlayerError(null)
      setPlayerLoading(false)
      return
    }

    if (!recordId) {
      setCastData(null)
      setRecordingBlob(null)
      setPlayerError(null)
      setPlayerLoading(false)
      return
    }

    let cancelled = false
    setPlayerLoading(true)
    setPlayerError(null)

    downloadSessionRecording(recordId)
      .then(async (blob) => {
        if (cancelled) {
          return
        }
        setRecordingBlob(blob)
        const buffer = await blob.arrayBuffer()
        const decompressed = ungzip(new Uint8Array(buffer), { to: 'string' }) as string | Uint8Array
        const castString =
          typeof decompressed === 'string' ? decompressed : new TextDecoder().decode(decompressed)
        if (!cancelled) {
          setCastData(castString)
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setPlayerError(error instanceof Error ? error.message : 'Failed to load recording')
          setCastData(null)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setPlayerLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [open, recordId])

  const startedLabel = useMemo(() => {
    if (!status?.started_at) {
      return '—'
    }
    return formatDistanceToNow(new Date(status.started_at), { addSuffix: true })
  }, [status?.started_at])

  const lastEventLabel = useMemo(() => {
    if (!status?.last_event_at) {
      return '—'
    }
    return formatDistanceToNow(new Date(status.last_event_at), { addSuffix: true })
  }, [status?.last_event_at])

  const capturedLabel = useMemo(() => {
    const createdAt = status?.record?.created_at
    if (!createdAt) {
      return undefined
    }
    return formatDistanceToNow(new Date(createdAt), { addSuffix: true })
  }, [status?.record?.created_at])

  const retentionLabel = useMemo(() => {
    const retention = status?.record?.retention_until
    if (!retention) {
      return undefined
    }
    return formatDistanceToNow(new Date(retention), { addSuffix: true })
  }, [status?.record?.retention_until])

  const handleDownload = async () => {
    if (!recordId) {
      return
    }
    try {
      setDownloading(true)
      const blob = recordingBlob ?? (await downloadSessionRecording(recordId))
      if (!recordingBlob) {
        setRecordingBlob(blob)
      }
      const url = URL.createObjectURL(blob)
      const filename = `${sessionId}-${recordId}.cast.gz`
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = filename
      anchor.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (error) {
      setPlayerError(error instanceof Error ? error.message : 'Failed to download recording')
    } finally {
      setDownloading(false)
    }
  }

  const badgeVariant = status?.active ? 'destructive' : status?.record ? 'secondary' : 'outline'
  const badgeLabel = status?.active
    ? 'Recording'
    : status?.record
      ? 'Available'
      : isLoading
        ? 'Checking'
        : 'Inactive'

  let recordingContent: ReactNode
  if (status?.record) {
    recordingContent = (
      <div className="space-y-3">
        <div className="flex flex-wrap gap-4 text-xs text-muted-foreground">
          <span>Duration: {formatDuration(status.record.duration_seconds)}</span>
          <span>Size: {formatBytes(status.record.size_bytes)}</span>
          {capturedLabel ? <span>Captured {capturedLabel}</span> : null}
          {retentionLabel ? <span>Purges {retentionLabel}</span> : null}
        </div>

        <div className="rounded-lg border border-border bg-muted/10 p-3">
          {playerLoading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading recording…
            </div>
          ) : playerError ? (
            <div className="text-sm text-destructive">{playerError}</div>
          ) : castData ? (
            <RecordingPlayer cast={castData} />
          ) : (
            <div className="text-sm text-muted-foreground">
              Recording metadata ready. Use the controls below to download or refresh.
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onRefresh?.()}
            disabled={isLoading || !onRefresh}
            className="flex items-center gap-1"
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
          <Button
            onClick={handleDownload}
            size="sm"
            className="flex items-center gap-1"
            disabled={downloading}
          >
            {downloading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Download className="h-4 w-4" />
            )}
            Download
          </Button>
        </div>
      </div>
    )
  } else if (status?.active) {
    recordingContent = (
      <div className="rounded-lg border border-border bg-muted/20 p-3 text-sm text-muted-foreground">
        Recording in progress. Last activity {lastEventLabel}. Captured{' '}
        {formatBytes(status.bytes_recorded)} so far.
      </div>
    )
  } else {
    recordingContent = (
      <div className="rounded-lg border border-dashed border-border/60 bg-muted/10 p-3 text-sm text-muted-foreground">
        Recording is not active for this session. Launch the connection with recording enabled to
        capture terminal output.
      </div>
    )
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Session recording"
      description="Review capture status and play back recorded SSH activity."
      size="xl"
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <span>Session</span>
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{sessionId}</code>
            </div>
            <p className="text-sm text-muted-foreground">Started {startedLabel}</p>
            {status?.recording_mode ? (
              <p className="text-xs text-muted-foreground uppercase tracking-wide">
                Policy: {status.recording_mode}
              </p>
            ) : null}
          </div>
          <Badge variant={badgeVariant} className="text-xs font-semibold">
            {badgeLabel}
          </Badge>
        </div>

        {recordingContent}
      </div>
    </Modal>
  )
}
