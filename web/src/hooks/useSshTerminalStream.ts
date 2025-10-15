import { useCallback, useMemo } from 'react'
import { buildWebSocketUrl } from '@/lib/utils/websocket'
import type { RealtimeMessage } from '@/types/realtime'
import { REALTIME_STREAM_SSH_TERMINAL } from '@/types/realtime'
import type { SshTerminalEvent } from '@/types/ssh'
import { useAuth } from './useAuth'
import { useWebSocket } from './useWebSocket'

interface RawTerminalPayload extends Record<string, unknown> {
  session_id?: string
  connection_id?: string
  payload?: string
  encoding?: string
  message?: string
  channel?: string
  type?: string
  error?: string
}

export interface UseSshTerminalStreamOptions {
  sessionId?: string
  enabled?: boolean
  onEvent?: (event: SshTerminalEvent) => void
}

function decodeBase64(base64: string): Uint8Array | null {
  if (!base64) {
    return null
  }
  if (typeof window !== 'undefined' && typeof window.atob === 'function') {
    try {
      const binary = window.atob(base64)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i += 1) {
        bytes[i] = binary.charCodeAt(i)
      }
      return bytes
    } catch {
      return null
    }
  }
  const bufferCtor = (
    globalThis as {
      Buffer?: { from: (input: string, enc: string) => { length: number; [index: number]: number } }
    }
  ).Buffer
  if (bufferCtor && typeof bufferCtor.from === 'function') {
    try {
      const buffer = bufferCtor.from(base64, 'base64')
      const bytes = new Uint8Array(buffer.length)
      for (let i = 0; i < buffer.length; i += 1) {
        bytes[i] = buffer[i]
      }
      return bytes
    } catch {
      return null
    }
  }
  return null
}

function bytesToUtf8(bytes: Uint8Array | null): string | undefined {
  if (!bytes || bytes.length === 0) {
    return undefined
  }
  try {
    const decoder = new TextDecoder()
    return decoder.decode(bytes)
  } catch {
    return undefined
  }
}

export function mapTerminalMessage(
  message: RealtimeMessage<RawTerminalPayload>,
  sessionId?: string
): SshTerminalEvent | null {
  if (!message || message.stream !== REALTIME_STREAM_SSH_TERMINAL) {
    return null
  }

  const payload = message.data ?? {}
  const resolvedSessionId = typeof payload.session_id === 'string' ? payload.session_id.trim() : ''
  if (!resolvedSessionId) {
    return null
  }
  if (sessionId && resolvedSessionId !== sessionId) {
    return null
  }

  const encoding = typeof payload.encoding === 'string' ? payload.encoding.toLowerCase() : undefined
  const rawPayload = typeof payload.payload === 'string' ? payload.payload : undefined
  const bytes = encoding === 'base64' && rawPayload ? decodeBase64(rawPayload) : undefined

  return {
    stream: message.stream,
    event: message.event,
    sessionId: resolvedSessionId,
    connectionId: payload.connection_id,
    message: payload.message ?? payload.error ?? undefined,
    channel: payload.channel ?? payload.type,
    encoding,
    raw: bytes ?? undefined,
    text: encoding === 'base64' ? bytesToUtf8(bytes ?? null) : rawPayload,
    original: payload,
  }
}

export function useSshTerminalStream({
  sessionId,
  enabled = true,
  onEvent,
}: UseSshTerminalStreamOptions) {
  const { tokens, isAuthenticated } = useAuth({ autoInitialize: true })
  const accessToken = tokens?.accessToken ?? ''

  const websocketUrl = useMemo(() => {
    if (!enabled || !isAuthenticated || !accessToken) {
      return ''
    }
    return buildWebSocketUrl('/ws', {
      token: accessToken,
      streams: REALTIME_STREAM_SSH_TERMINAL,
    })
  }, [accessToken, enabled, isAuthenticated])

  const handleMessage = useCallback(
    (message: RealtimeMessage<RawTerminalPayload> | null) => {
      if (!message) {
        return
      }
      const mapped = mapTerminalMessage(message, sessionId)
      if (!mapped) {
        return
      }
      onEvent?.(mapped)
    },
    [onEvent, sessionId]
  )

  return useWebSocket<RealtimeMessage<RawTerminalPayload>>(websocketUrl, {
    enabled: Boolean(websocketUrl),
    autoReconnect: true,
    onMessage: handleMessage,
  })
}
