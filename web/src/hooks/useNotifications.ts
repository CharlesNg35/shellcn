import { useCallback, useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { notificationsApi } from '@/lib/api/notifications'
import { buildWebSocketUrl } from '@/lib/utils/websocket'
import type { NotificationEventData, NotificationPayload } from '@/types/notifications'
import type { RealtimeMessage } from '@/types/realtime'
import { REALTIME_STREAM_NOTIFICATIONS } from '@/types/realtime'
import { useAuth } from './useAuth'
import { useWebSocket } from './useWebSocket'

export const NOTIFICATION_QUERY_KEY = ['notifications', 'list'] as const

function mergeNotification(
  existing: NotificationPayload[],
  notification: NotificationPayload
): NotificationPayload[] {
  const next = existing.slice()
  const index = next.findIndex((item) => item.id === notification.id)

  if (index >= 0) {
    next[index] = notification
  } else {
    next.unshift(notification)
  }

  return next
}

function removeNotification(existing: NotificationPayload[], notificationId: string) {
  return existing.filter((item) => item.id !== notificationId)
}

export function useNotifications() {
  const { isAuthenticated, tokens } = useAuth({ autoInitialize: true })
  const queryClient = useQueryClient()

  const notificationsQuery = useQuery({
    queryKey: NOTIFICATION_QUERY_KEY,
    queryFn: () => notificationsApi.list({ limit: 25 }),
    enabled: isAuthenticated,
    staleTime: 30 * 1000,
    refetchInterval: isAuthenticated ? 60 * 1000 : false,
  })

  const notifications = useMemo(() => notificationsQuery.data ?? [], [notificationsQuery.data])

  const handleSocketMessage = useCallback(
    (message: RealtimeMessage<NotificationEventData> | null) => {
      if (!message || message.stream !== REALTIME_STREAM_NOTIFICATIONS) {
        return
      }

      queryClient.setQueryData<NotificationPayload[]>(NOTIFICATION_QUERY_KEY, (current = []) => {
        const { event: eventType, data } = message
        const notificationData = data?.notification
        const notificationId = data?.notification_id ?? notificationData?.id

        switch (eventType) {
          case 'notification.created':
          case 'notification.updated':
            if (notificationData) {
              return mergeNotification(current, notificationData)
            }
            return current
          case 'notification.read':
            if (notificationId) {
              return current.map((item) =>
                item.id === notificationId ? { ...item, is_read: true } : item
              )
            }
            if (notificationData) {
              return mergeNotification(current, { ...notificationData, is_read: true })
            }
            return current
          case 'notification.deleted':
            if (notificationId) {
              return removeNotification(current, notificationId)
            }
            return current
          case 'notification.read_all':
            return current.map((item) => ({ ...item, is_read: true }))
          default:
            return current
        }
      })
    },
    [queryClient]
  )

  const accessToken = tokens?.accessToken ?? ''

  const websocketUrl = useMemo(() => {
    if (!isAuthenticated || !accessToken) {
      return ''
    }
    return buildWebSocketUrl('/ws', {
      token: accessToken,
      streams: REALTIME_STREAM_NOTIFICATIONS,
    })
  }, [accessToken, isAuthenticated])

  const { isConnected } = useWebSocket<RealtimeMessage<NotificationEventData>>(websocketUrl, {
    enabled: isAuthenticated && Boolean(websocketUrl),
    autoReconnect: true,
    onMessage: handleSocketMessage,
  })

  const markReadMutation = useMutation({
    mutationFn: (notificationId: string) => notificationsApi.markAsRead(notificationId),
    onSuccess: (_data, notificationId) => {
      queryClient.setQueryData<NotificationPayload[]>(NOTIFICATION_QUERY_KEY, (current = []) =>
        current.map((item) => (item.id === notificationId ? { ...item, is_read: true } : item))
      )
    },
  })

  const markUnreadMutation = useMutation({
    mutationFn: (notificationId: string) => notificationsApi.markAsUnread(notificationId),
    onSuccess: (_data, notificationId) => {
      queryClient.setQueryData<NotificationPayload[]>(NOTIFICATION_QUERY_KEY, (current = []) =>
        current.map((item) => (item.id === notificationId ? { ...item, is_read: false } : item))
      )
    },
  })

  const removeMutation = useMutation({
    mutationFn: (notificationId: string) => notificationsApi.remove(notificationId),
    onSuccess: (_data, notificationId) => {
      queryClient.setQueryData<NotificationPayload[]>(NOTIFICATION_QUERY_KEY, (current = []) =>
        removeNotification(current, notificationId)
      )
    },
  })

  const markAllMutation = useMutation({
    mutationFn: () => notificationsApi.markAllAsRead(),
    onSuccess: () => {
      queryClient.setQueryData<NotificationPayload[]>(NOTIFICATION_QUERY_KEY, (current = []) =>
        current.map((item) => ({ ...item, is_read: true }))
      )
    },
  })

  const unreadCount = useMemo(
    () => notifications.filter((notification) => !notification.is_read).length,
    [notifications]
  )

  return {
    notifications,
    unreadCount,
    isLoading: notificationsQuery.isLoading,
    isConnected,
    refetch: notificationsQuery.refetch,
    markAsRead: markReadMutation.mutateAsync,
    markAsUnread: markUnreadMutation.mutateAsync,
    removeNotification: removeMutation.mutateAsync,
    markAllAsRead: markAllMutation.mutateAsync,
  }
}
