import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
import type { TransferItem } from './types'
import { formatBytes, formatLabel } from './utils'

interface TransferSidebarProps {
  transfers: TransferItem[]
  onClear: () => void
}

export function TransferSidebar({ transfers, onClear }: TransferSidebarProps) {
  return (
    <aside className="w-full max-w-xs space-y-2 rounded-lg border border-border bg-card/50 p-3 shadow-sm">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-medium text-muted-foreground">Transfers</h3>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClear}
          disabled={!transfers.some((transfer) => transfer.status !== 'uploading')}
          className="h-6 text-xs"
        >
          Clear
        </Button>
      </div>

      {transfers.length === 0 ? (
        <div className="rounded-md border border-dashed border-border/70 p-3 text-xs text-muted-foreground">
          No active transfers
        </div>
      ) : (
        <ul className="space-y-2 overflow-y-auto">
          {transfers.map((transfer) => {
            const progress = transfer.size
              ? Math.min(transfer.uploaded / transfer.size, 1)
              : transfer.uploaded > 0
                ? 1
                : 0

            return (
              <li
                key={transfer.id}
                className="rounded-md border border-border/60 bg-background/60 p-2.5 shadow-sm"
              >
                <div className="flex items-center justify-between text-xs font-medium">
                  <span className="truncate">{transfer.name}</span>
                  <span className="text-[10px] text-muted-foreground">
                    {formatBytes(transfer.size)}
                  </span>
                </div>
                <div className="mt-1.5 h-1.5 rounded-full bg-muted">
                  <div
                    className={cn('h-1.5 rounded-full bg-primary transition-all', {
                      'bg-destructive': transfer.status === 'failed',
                    })}
                    style={{ width: `${progress * 100}%` }}
                  />
                </div>
                <div className="mt-1.5 flex flex-wrap items-center justify-between gap-1.5 text-[10px] text-muted-foreground">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="rounded-full bg-muted px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide">
                      {formatLabel(transfer.direction)}
                    </span>
                    <span className="capitalize">{formatLabel(transfer.status)}</span>
                    {transfer.userName && (
                      <span className="text-muted-foreground/70">· {transfer.userName}</span>
                    )}
                  </div>
                  <span>
                    {transfer.uploaded === transfer.size && transfer.size > 0
                      ? formatBytes(transfer.size)
                      : `${formatBytes(transfer.uploaded)} / ${formatBytes(transfer.size)}`}
                  </span>
                </div>
                {transfer.errorMessage && (
                  <p className="mt-1.5 text-[10px] text-destructive">{transfer.errorMessage}</p>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </aside>
  )
}
