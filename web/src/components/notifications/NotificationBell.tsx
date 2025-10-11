import { useCallback, useEffect, useRef, useState } from 'react'
import { Bell } from 'lucide-react'
import { useNotifications } from '@/hooks/useNotifications'
import type { NotificationPayload } from '@/types/notifications'
import { NotificationCenter } from './NotificationCenter'
import { cn } from '@/lib/utils/cn'

interface NotificationBellProps {
  className?: string
}

export function NotificationBell({ className }: NotificationBellProps) {
  const [isOpen, setIsOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const {
    notifications,
    unreadCount,
    isConnected,
    isLoading,
    markAsRead,
    markAsUnread,
    removeNotification,
    markAllAsRead,
  } = useNotifications()

  const toggleOpen = () => setIsOpen((prev) => !prev)

  const handleToggleRead = useCallback(
    async (notification: NotificationPayload) => {
      if (notification.is_read) {
        await markAsUnread(notification.id)
      } else {
        await markAsRead(notification.id)
      }
    },
    [markAsRead, markAsUnread]
  )

  const handleRemove = useCallback(
    async (notification: NotificationPayload) => {
      await removeNotification(notification.id)
    },
    [removeNotification]
  )

  const handleNavigate = useCallback(
    (notification: NotificationPayload) => {
      if (!notification.action_url || typeof window === 'undefined') {
        return
      }
      window.open(notification.action_url, '_blank', 'noopener,noreferrer')
      void markAsRead(notification.id)
      setIsOpen(false)
    },
    [markAsRead]
  )

  useEffect(() => {
    if (!isOpen) {
      return
    }

    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    const handleEsc = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEsc)

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEsc)
    }
  }, [isOpen])

  return (
    <div className={cn('relative', className)} ref={containerRef}>
      <button
        type="button"
        onClick={toggleOpen}
        className={cn(
          'relative rounded-full p-2 text-muted-foreground transition hover:bg-muted hover:text-foreground',
          isOpen && 'bg-muted text-foreground'
        )}
        aria-label="Notifications"
        aria-expanded={isOpen}
      >
        <Bell className="h-5 w-5" aria-hidden="true" />
        {unreadCount > 0 ? (
          <span className="absolute -right-0.5 -top-0.5 inline-flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-semibold text-destructive-foreground">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        ) : null}
      </button>

      {isOpen ? (
        <div className="absolute right-0 top-10 z-50">
          <NotificationCenter
            notifications={notifications}
            unreadCount={unreadCount}
            isConnected={isConnected}
            isLoading={isLoading}
            onMarkAllRead={() => {
              void markAllAsRead()
            }}
            onToggleRead={handleToggleRead}
            onRemove={handleRemove}
            onNavigate={handleNavigate}
          />
        </div>
      ) : null}
    </div>
  )
}
