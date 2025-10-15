export interface RealtimeMessage<TData = unknown> {
  stream: string
  event: string
  data?: TData
  meta?: Record<string, unknown>
}

export type RealtimeControlAction = 'subscribe' | 'unsubscribe' | 'ping'

export interface RealtimeControlMessage {
  action: RealtimeControlAction
  streams?: string[]
}

export const REALTIME_STREAM_NOTIFICATIONS = 'notifications'
export const REALTIME_STREAM_SFTP = 'ssh.sftp'
