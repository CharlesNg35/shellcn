import type { NotificationPayload } from '@/types/notifications'
import { NotificationItem } from './NotificationItem'
import { cn } from '@/lib/utils/cn'

interface NotificationCenterProps {
  notifications: NotificationPayload[]
  unreadCount: number
  isConnected: boolean
  isLoading?: boolean
  onMarkAllRead?: () => void
  onToggleRead?: (notification: NotificationPayload) => void
  onRemove?: (notification: NotificationPayload) => void
  onNavigate?: (notification: NotificationPayload) => void
  className?: string
}

export function NotificationCenter({
  notifications,
  unreadCount,
  isConnected,
  isLoading = false,
  onMarkAllRead,
  onToggleRead,
  onRemove,
  onNavigate,
  className,
}: NotificationCenterProps) {
  return (
    <div className={cn('w-80 rounded-lg border border-border bg-popover shadow-xl', className)}>
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <p className="text-sm font-semibold text-popover-foreground">Notifications</p>
          <p className="text-xs text-muted-foreground">
            {isConnected ? 'Live updates enabled' : 'Offline – retrying connection'}
          </p>
        </div>
        <button
          className="text-xs font-medium text-primary hover:underline disabled:cursor-not-allowed disabled:opacity-60"
          onClick={onMarkAllRead}
          disabled={unreadCount === 0 || isLoading}
        >
          Mark all read
        </button>
      </div>

      <div className="max-h-96 space-y-2 overflow-y-auto p-3">
        {isLoading ? (
          <div className="space-y-2">
            {[0, 1, 2].map((index) => (
              <div key={index} className="animate-pulse rounded-lg border border-border p-4">
                <div className="h-4 w-2/3 rounded bg-muted" />
                <div className="mt-2 h-3 w-full rounded bg-muted" />
              </div>
            ))}
          </div>
        ) : notifications.length ? (
          notifications.map((notification) => (
            <NotificationItem
              key={notification.id}
              notification={notification}
              onToggleRead={onToggleRead}
              onRemove={onRemove}
              onNavigate={onNavigate}
            />
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-border p-6 text-center">
            <p className="text-sm font-medium text-muted-foreground">No notifications yet</p>
            <p className="mt-1 text-xs text-muted-foreground">
              You’ll see alerts about sessions, permissions, and system activity here.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
