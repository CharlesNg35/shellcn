export type NotificationSeverity = 'info' | 'success' | 'warning' | 'error'

export interface NotificationPayload {
  id: string
  type: string
  title: string
  message: string
  created_at: string
  is_read: boolean
  action_url?: string | null
  metadata?: Record<string, unknown>
  severity?: NotificationSeverity
}

export interface NotificationEventData {
  notification?: NotificationPayload
  notification_id?: string
}
