import { formatDistanceToNow } from 'date-fns'
import { CheckCircle2, CircleDot, ExternalLink, Trash2 } from 'lucide-react'
import type { NotificationPayload } from '@/types/notifications'
import { cn } from '@/lib/utils/cn'

interface NotificationItemProps {
  notification: NotificationPayload
  onToggleRead?: (notification: NotificationPayload) => void
  onRemove?: (notification: NotificationPayload) => void
  onNavigate?: (notification: NotificationPayload) => void
}

export function NotificationItem({
  notification,
  onToggleRead,
  onRemove,
  onNavigate,
}: NotificationItemProps) {
  const handleToggleRead = () => {
    onToggleRead?.(notification)
  }

  const handleRemove = () => {
    onRemove?.(notification)
  }

  const handleNavigate = () => {
    if (notification.action_url) {
      onNavigate?.(notification)
    }
  }

  const relativeTime = formatDistanceToNow(new Date(notification.created_at), {
    addSuffix: true,
  })

  return (
    <div
      className={cn(
        'group rounded-lg border border-border bg-card/60 p-3 transition hover:bg-card',
        notification.is_read ? 'opacity-70' : 'border-primary/40'
      )}
    >
      <div className="flex items-start gap-3">
        <div className="mt-1">
          {notification.is_read ? (
            <CheckCircle2 className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
          ) : (
            <CircleDot className="h-4 w-4 text-primary" aria-hidden="true" />
          )}
        </div>
        <div className="flex-1 space-y-1">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-foreground">{notification.title}</p>
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {notification.type}
              </p>
            </div>
            <span className="text-xs text-muted-foreground">{relativeTime}</span>
          </div>
          {notification.message ? (
            <p className="text-sm text-muted-foreground">{notification.message}</p>
          ) : null}
          <div className="flex flex-wrap gap-2 pt-1">
            <button
              onClick={handleToggleRead}
              className="text-xs font-medium text-primary hover:underline"
            >
              {notification.is_read ? 'Mark as unread' : 'Mark as read'}
            </button>
            {notification.action_url ? (
              <button
                onClick={handleNavigate}
                className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
              >
                View
                <ExternalLink className="h-3 w-3" />
              </button>
            ) : null}
            <button
              onClick={handleRemove}
              className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="h-3 w-3" />
              Remove
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
