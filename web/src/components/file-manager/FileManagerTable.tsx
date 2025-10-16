import { format } from 'date-fns'
import { Download, MoreVertical, Trash2 } from 'lucide-react'
import type { ReactNode, RefObject } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Button } from '@/components/ui/Button'
import type { SftpEntry } from '@/types/sftp'
import { displayPath, formatBytes } from './utils'
import { cn } from '@/lib/utils/cn'

interface FileManagerTableProps {
  entries: SftpEntry[]
  onActivate: (entry: SftpEntry) => void
  onDownload: (entry: SftpEntry) => void
  onDelete: (entry: SftpEntry) => void
  canWrite: boolean
  renderIcon: (entry: SftpEntry) => ReactNode
  scrollContainerRef: RefObject<HTMLElement | null>
}

const ROW_HEIGHT = 56
const GRID_TEMPLATE_CLASS =
  'grid-cols-[minmax(220px,2.5fr)_minmax(120px,1fr)_minmax(160px,1fr)_minmax(120px,1fr)_minmax(140px,1fr)]'

export function FileManagerTable({
  entries,
  onActivate,
  onDownload,
  onDelete,
  canWrite,
  renderIcon,
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
      <div role="rowgroup" className="sticky top-0 z-10 bg-muted/70 backdrop-blur">
        <div
          role="row"
          className={cn(
            'grid items-center px-4 py-2 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground',
            GRID_TEMPLATE_CLASS
          )}
        >
          <span role="columnheader">Name</span>
          <span role="columnheader">Size</span>
          <span role="columnheader">Modified</span>
          <span role="columnheader">Mode</span>
          <span role="columnheader" className="text-right">
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
                'group absolute left-0 right-0 grid cursor-pointer border-b border-border/80 bg-background/40 hover:bg-muted/40',
                GRID_TEMPLATE_CLASS
              )}
              style={{ transform: `translateY(${virtualRow.start}px)`, height: virtualRow.size }}
              onDoubleClick={() => onActivate(entry)}
            >
              <div role="cell" className="flex items-center gap-3 px-4 py-2">
                {renderIcon(entry)}
                <div className="flex flex-col">
                  <span
                    className={cn('font-medium text-foreground', { 'text-primary': isDirectory })}
                  >
                    {entry.name}
                  </span>
                  <span className="text-xs text-muted-foreground">{displayPath(entry.path)}</span>
                </div>
              </div>
              <div role="cell" className="px-4 py-2 text-muted-foreground">
                {isDirectory ? '—' : formatBytes(entry.size)}
              </div>
              <div role="cell" className="px-4 py-2 text-muted-foreground">
                {format(entry.modifiedAt, 'yyyy-MM-dd HH:mm')}
              </div>
              <div role="cell" className="px-4 py-2 text-muted-foreground">
                {entry.mode}
              </div>
              <div role="cell" className="px-4 py-2">
                <div className="flex justify-end gap-1 opacity-0 transition group-hover:opacity-100">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Download"
                    onClick={(event) => {
                      event.stopPropagation()
                      onDownload(entry)
                    }}
                    disabled={isDirectory}
                  >
                    <Download className="h-4 w-4" aria-hidden />
                  </Button>
                  {canWrite && (
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Delete"
                      onClick={(event) => {
                        event.stopPropagation()
                        onDelete(entry)
                      }}
                    >
                      <Trash2 className="h-4 w-4 text-destructive" aria-hidden />
                    </Button>
                  )}
                  <Button variant="ghost" size="icon" aria-label="More actions" disabled>
                    <MoreVertical className="h-4 w-4 text-muted-foreground" aria-hidden />
                  </Button>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
