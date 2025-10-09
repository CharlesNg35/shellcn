import type { ApiResponse } from '@/types/api'
import type { NotificationPayload } from '@/types/notifications'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const NOTIFICATIONS_ENDPOINT = '/notifications'
const NOTIFICATIONS_READ_ALL_ENDPOINT = '/notifications/read-all'

interface ListNotificationsParams {
  limit?: number
  offset?: number
}

export async function listNotifications(
  params?: ListNotificationsParams
): Promise<NotificationPayload[]> {
  const response = await apiClient.get<ApiResponse<NotificationPayload[]>>(NOTIFICATIONS_ENDPOINT, {
    params,
  })
  return unwrapResponse(response)
}

export async function markNotificationAsRead(notificationId: string): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(
    `${NOTIFICATIONS_ENDPOINT}/${notificationId}/read`
  )
  unwrapResponse(response)
}

export async function markNotificationAsUnread(notificationId: string): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(
    `${NOTIFICATIONS_ENDPOINT}/${notificationId}/unread`
  )
  unwrapResponse(response)
}

export async function clearNotification(notificationId: string): Promise<void> {
  const response = await apiClient.delete<ApiResponse<unknown>>(
    `${NOTIFICATIONS_ENDPOINT}/${notificationId}`
  )
  unwrapResponse(response)
}

export async function markAllNotificationsRead(): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(NOTIFICATIONS_READ_ALL_ENDPOINT)
  unwrapResponse(response)
}

export const notificationsApi = {
  list: listNotifications,
  markAsRead: markNotificationAsRead,
  markAsUnread: markNotificationAsUnread,
  remove: clearNotification,
  markAllAsRead: markAllNotificationsRead,
}
