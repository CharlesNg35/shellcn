import { Button } from '@/components/ui/Button'
import { formatDistanceToNow } from 'date-fns'
import { cn } from '@/lib/utils/cn'
import { Loader2, Search as SearchIcon } from 'lucide-react'
import type { RefObject } from 'react'
import type { SessionRecordingStatus } from '@/types/session-recording'

interface SshWorkspaceStatusBarProps {
  fontSize: number
  onZoomIn: () => void
  onZoomOut: () => void
  onZoomReset: () => void
  onToggleSearch: () => void
  isSearchOpen: boolean
  searchQuery: string
  onSearchQueryChange: (value: string) => void
  searchDirection: 'next' | 'previous'
  onSearchDirectionChange: (direction: 'next' | 'previous') => void
  onSearchSubmit: () => void
  searchMatched: boolean
  latencyMs: number | null
  lastActivityAt: Date | null
  transfers: { active: number; total: number }
  recordingActive: boolean
  recordingStatus?: SessionRecordingStatus
  recordingLoading?: boolean
  onRecordingDetails?: () => void
  searchInputRef?: RefObject<HTMLInputElement>
}

export function SshWorkspaceStatusBar({
  fontSize,
  onZoomIn,
  onZoomOut,
  onZoomReset,
  onToggleSearch,
  isSearchOpen,
  searchQuery,
  onSearchQueryChange,
  searchDirection,
  onSearchDirectionChange,
  onSearchSubmit,
  searchMatched,
  latencyMs,
  lastActivityAt,
  transfers,
  recordingActive,
  recordingStatus,
  recordingLoading,
  onRecordingDetails,
  searchInputRef,
}: SshWorkspaceStatusBarProps) {
  const lastActivityLabel = lastActivityAt
    ? formatDistanceToNow(lastActivityAt, { addSuffix: true })
    : '—'

  const showRecordingIndicator =
    recordingLoading || recordingStatus?.active || recordingStatus?.record || recordingActive

  const recordingLabel = recordingLoading
    ? 'Checking…'
    : recordingStatus?.active || recordingActive
      ? 'Recording…'
      : recordingStatus?.record
        ? 'Recording saved'
        : null

  const recordingIndicator =
    showRecordingIndicator && recordingLabel ? (
      <div className="flex items-center">
        {onRecordingDetails ? (
          <button
            type="button"
            onClick={onRecordingDetails}
            className="flex items-center gap-1 rounded-md border border-rose-500/30 bg-rose-500/10 px-2 py-1 text-rose-500 transition-colors hover:bg-rose-500/20"
            disabled={recordingLoading}
          >
            {recordingLoading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <span
                className={cn(
                  'h-2 w-2 rounded-full bg-rose-500',
                  (recordingStatus?.active || recordingActive) && 'animate-pulse'
                )}
              />
            )}
            <span>{recordingLabel}</span>
          </button>
        ) : (
          <span className="flex items-center gap-1 text-rose-500">
            {recordingLoading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <span
                className={cn(
                  'h-2 w-2 rounded-full bg-rose-500',
                  (recordingStatus?.active || recordingActive) && 'animate-pulse'
                )}
              />
            )}
            <span>{recordingLabel}</span>
          </span>
        )}
      </div>
    ) : null

  return (
    <div className="flex flex-col gap-2 border-t border-border/60 bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onZoomOut} aria-label="Zoom out">
            -
          </Button>
          <span className="font-mono text-sm">{fontSize}px</span>
          <Button variant="ghost" size="sm" onClick={onZoomIn} aria-label="Zoom in">
            +
          </Button>
          <Button variant="ghost" size="sm" onClick={onZoomReset} aria-label="Reset zoom">
            Reset
          </Button>
        </div>

        <div className="flex flex-wrap items-center gap-4">
          <button
            type="button"
            className={cn(
              'flex items-center gap-1 rounded-md px-2 py-1 transition-colors',
              isSearchOpen ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
            )}
            onClick={onToggleSearch}
          >
            <SearchIcon className="h-4 w-4" />
            Search
          </button>
          <span>Latency: {latencyMs != null ? `${Math.round(latencyMs)} ms` : '—'}</span>
          <span>Last activity: {lastActivityLabel}</span>
          <span>
            Transfers: {transfers.active}/{transfers.total}
          </span>
          {recordingIndicator}
        </div>
      </div>

      {isSearchOpen && (
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            onSearchSubmit()
          }}
        >
          <input
            ref={searchInputRef}
            value={searchQuery}
            onChange={(event) => onSearchQueryChange(event.target.value)}
            placeholder="Search terminal output"
            className="max-w-xs rounded-md border border-border bg-background px-2 py-1 text-sm text-foreground shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant={searchDirection === 'next' ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => onSearchDirectionChange('next')}
            >
              Next
            </Button>
            <Button
              type="button"
              variant={searchDirection === 'previous' ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => onSearchDirectionChange('previous')}
            >
              Previous
            </Button>
          </div>
          <Button type="submit" size="sm" disabled={!searchQuery}>
            Find
          </Button>
          {!searchMatched && searchQuery && (
            <span className="text-xs text-rose-500">No matches</span>
          )}
        </form>
      )}
    </div>
  )
}

export default SshWorkspaceStatusBar
