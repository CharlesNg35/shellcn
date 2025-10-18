import { format } from 'date-fns'
import { Download, Trash2, File, FolderOpen } from 'lucide-react'
import type { RefObject } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Button } from '@/components/ui/Button'
import type { SftpEntry } from '@/types/sftp'
import { formatBytes } from './utils'
import { cn } from '@/lib/utils/cn'

interface FileManagerTableProps {
  entries: SftpEntry[]
  onActivate: (entry: SftpEntry) => void
  onDownload: (entry: SftpEntry) => void
  onDelete: (entry: SftpEntry) => void
  canWrite: boolean
  scrollContainerRef: RefObject<HTMLElement | null>
}

const ROW_HEIGHT = 48
const GRID_TEMPLATE_CLASS =
  'grid-cols-[minmax(280px,3fr)_minmax(100px,0.8fr)_minmax(140px,1.2fr)_minmax(100px,0.8fr)_80px]'

export function FileManagerTable({
  entries,
  onActivate,
  onDownload,
  onDelete,
  canWrite,
  scrollContainerRef,
}: FileManagerTableProps) {
  const virtualizer = useVirtualizer({
    count: entries.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8,
    initialRect: { width: 0, height: 480 },
  })

  const virtualRows = virtualizer.getVirtualItems()
  const totalHeight = virtualizer.getTotalSize()

  if (!entries.length) {
    return null
  }

  return (
    <div role="table" aria-rowcount={entries.length} className="min-w-full text-sm">
      <div
        role="rowgroup"
        className="sticky top-0 z-10 border-b border-border/50 bg-muted/50 backdrop-blur-sm"
      >
        <div
          role="row"
          className={cn(
            'grid items-center gap-3 px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted-foreground',
            GRID_TEMPLATE_CLASS
          )}
        >
          <span role="columnheader">Name</span>
          <span role="columnheader">Size</span>
          <span role="columnheader">Modified</span>
          <span role="columnheader">Mode</span>
          <span role="columnheader" className="text-center">
            Actions
          </span>
        </div>
      </div>
      <div role="rowgroup" className="relative" style={{ height: totalHeight }}>
        {virtualRows.map((virtualRow) => {
          const entry = entries[virtualRow.index]
          const isDirectory = entry.isDir
          return (
            <div
              key={entry.path}
              role="row"
              className={cn(
                'group absolute left-0 right-0 grid cursor-pointer items-center gap-3 border-b border-border/40 bg-background/60 px-4 transition-colors hover:bg-accent/50',
                GRID_TEMPLATE_CLASS
              )}
              style={{ transform: `translateY(${virtualRow.start}px)`, height: virtualRow.size }}
              onDoubleClick={() => onActivate(entry)}
            >
              <div role="cell" className="flex min-w-0 items-center gap-2.5">
                {isDirectory ? (
                  <FolderOpen className="h-4 w-4 flex-shrink-0 text-blue-500" aria-hidden />
                ) : (
                  <File className="h-4 w-4 flex-shrink-0 text-slate-400" aria-hidden />
                )}
                <span
                  className={cn(
                    'truncate text-sm font-medium',
                    isDirectory ? 'text-foreground' : 'text-foreground'
                  )}
                  title={entry.name}
                >
                  {entry.name}
                </span>
              </div>
              <div role="cell" className="text-xs text-muted-foreground">
                {isDirectory ? '—' : formatBytes(entry.size ?? 0)}
              </div>
              <div role="cell" className="text-xs text-muted-foreground">
                {entry.modifiedAt ? format(new Date(entry.modifiedAt), 'MMM dd, yyyy HH:mm') : '—'}
              </div>
              <div role="cell" className="font-mono text-[11px] text-muted-foreground">
                {entry.mode || '—'}
              </div>
              <div role="cell" className="flex items-center justify-center">
                <div className="flex gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                  {!isDirectory && (
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Download"
                      onClick={(event) => {
                        event.stopPropagation()
                        onDownload(entry)
                      }}
                      className="h-7 w-7"
                    >
                      <Download className="h-3.5 w-3.5" aria-hidden />
                    </Button>
                  )}
                  {canWrite && (
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Delete"
                      onClick={(event) => {
                        event.stopPropagation()
                        onDelete(entry)
                      }}
                      className="h-7 w-7 hover:text-destructive"
                    >
                      <Trash2 className="h-3.5 w-3.5" aria-hidden />
                    </Button>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
