import { useCallback } from 'react'
import type { RealtimeMessage } from '@/types/realtime'
import { REALTIME_STREAM_SFTP } from '@/types/realtime'
import type { SftpTransferRealtimeEvent, SftpTransferStatus } from '@/types/sftp'
import { useAuthenticatedWebSocket } from './useAuthenticatedWebSocket'

interface RawSftpTransferPayload {
  session_id?: string
  connection_id?: string
  user_id?: string
  path?: string
  direction?: string
  transfer_id?: string
  status?: string
  bytes_transferred?: number
  total_bytes?: number
  error?: string
}

interface UseSftpTransfersStreamOptions {
  sessionId?: string
  enabled?: boolean
  onEvent?: (event: SftpTransferRealtimeEvent) => void
}

function mapStatus(eventType: string, rawStatus?: string): SftpTransferStatus {
  const extracted = (rawStatus ?? eventType.split('.').pop() ?? '').toLowerCase()
  switch (extracted) {
    case 'started':
    case 'progress':
    case 'completed':
    case 'failed':
      return extracted
    default:
      return extracted as SftpTransferStatus
  }
}

function mapDirection(rawDirection?: string) {
  if (!rawDirection) {
    return 'upload'
  }
  return rawDirection.toLowerCase() as SftpTransferRealtimeEvent['payload']['direction']
}

function mapPayload(
  eventType: string,
  raw: RawSftpTransferPayload
): SftpTransferRealtimeEvent | null {
  const transferId = raw.transfer_id?.trim()
  const sessionId = raw.session_id?.trim()
  const path = raw.path?.trim()
  if (!transferId || !sessionId || !path) {
    return null
  }

  const totalBytes =
    typeof raw.total_bytes === 'number' && raw.total_bytes >= 0 ? raw.total_bytes : undefined
  const bytesTransferred =
    typeof raw.bytes_transferred === 'number' && raw.bytes_transferred >= 0
      ? raw.bytes_transferred
      : undefined

  return {
    event: eventType,
    status: mapStatus(eventType, raw.status),
    payload: {
      sessionId,
      connectionId: raw.connection_id ?? undefined,
      userId: raw.user_id ?? undefined,
      path,
      direction: mapDirection(raw.direction),
      transferId,
      status: mapStatus(eventType, raw.status),
      bytesTransferred,
      totalBytes,
      error: raw.error ?? undefined,
    },
  }
}

export function useSftpTransfersStream({
  sessionId,
  enabled = true,
  onEvent,
}: UseSftpTransfersStreamOptions) {
  const handleMessage = useCallback(
    (message: RealtimeMessage<RawSftpTransferPayload> | null) => {
      if (!message || message.stream !== REALTIME_STREAM_SFTP) {
        return
      }
      const payload = message.data
      if (!payload) {
        return
      }
      if (sessionId && payload.session_id && payload.session_id !== sessionId) {
        return
      }
      const mapped = mapPayload(message.event, payload)
      if (!mapped) {
        return
      }
      onEvent?.(mapped)
    },
    [onEvent, sessionId]
  )

  const { isConnected, close, send, lastMessage } = useAuthenticatedWebSocket<
    RealtimeMessage<RawSftpTransferPayload>
  >({
    params: { streams: REALTIME_STREAM_SFTP },
    enabled,
    autoReconnect: true,
    onMessage: handleMessage,
  })

  return {
    isConnected,
    close,
    send,
    lastMessage,
  }
}
