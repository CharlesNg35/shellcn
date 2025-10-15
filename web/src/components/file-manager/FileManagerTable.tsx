import { format } from 'date-fns'
import { Download, MoreVertical, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
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
}

export function FileManagerTable({
  entries,
  onActivate,
  onDownload,
  onDelete,
  canWrite,
  renderIcon,
}: FileManagerTableProps) {
  if (!entries.length) {
    return null
  }

  return (
    <table className="min-w-full text-sm">
      <thead className="sticky top-0 z-10 bg-muted/70 backdrop-blur">
        <tr className="text-left">
          <th className="px-4 py-2 font-medium text-muted-foreground">Name</th>
          <th className="px-4 py-2 font-medium text-muted-foreground">Size</th>
          <th className="px-4 py-2 font-medium text-muted-foreground">Modified</th>
          <th className="px-4 py-2 font-medium text-muted-foreground">Mode</th>
          <th className="px-4 py-2 text-right font-medium text-muted-foreground">Actions</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((entry) => {
          const isDirectory = entry.isDir
          return (
            <tr
              key={entry.path}
              className="group cursor-pointer border-b border-border/80 hover:bg-muted/40"
              onDoubleClick={() => onActivate(entry)}
            >
              <td className="flex items-center gap-3 px-4 py-2">
                {renderIcon(entry)}
                <div className="flex flex-col">
                  <span
                    className={cn('font-medium text-foreground', { 'text-primary': isDirectory })}
                  >
                    {entry.name}
                  </span>
                  <span className="text-xs text-muted-foreground">{displayPath(entry.path)}</span>
                </div>
              </td>
              <td className="px-4 py-2 text-muted-foreground">
                {isDirectory ? '—' : formatBytes(entry.size)}
              </td>
              <td className="px-4 py-2 text-muted-foreground">
                {format(entry.modifiedAt, 'yyyy-MM-dd HH:mm')}
              </td>
              <td className="px-4 py-2 text-muted-foreground">{entry.mode}</td>
              <td className="px-4 py-2">
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
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}
