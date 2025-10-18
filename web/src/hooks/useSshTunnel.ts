/**
 * Hook for managing SSH tunnel WebSocket connection
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { SessionTunnelEntry } from '@/store/ssh-session-tunnel-store'
import type { SshTerminalEvent } from '@/types/ssh'
import { buildWebSocketUrl } from '@/lib/utils/websocket'
import { normalizeTerminalDimensions } from '@/lib/utils/terminal'

export type TunnelState = 'idle' | 'connecting' | 'open' | 'failed'

interface UseSshTunnelOptions {
  tunnel?: SessionTunnelEntry
  sessionId: string
  onEvent: (event: SshTerminalEvent) => void
}

interface UseSshTunnelResult {
  tunnelState: TunnelState
  sendInput: (data: string) => boolean
  sendResize: (cols: number, rows: number) => boolean
  close: () => void
}

export function useSshTunnel({
  tunnel,
  sessionId,
  onEvent,
}: UseSshTunnelOptions): UseSshTunnelResult {
  const [tunnelState, setTunnelState] = useState<TunnelState>(tunnel ? 'connecting' : 'idle')
  const socketRef = useRef<WebSocket | null>(null)
  const decoderRef = useRef<TextDecoder | null>(null)
  const lastSentResizeRef = useRef<{ cols: number; rows: number } | null>(null)

  // Update tunnel state when tunnel prop changes
  useEffect(() => {
    if (!tunnel) {
      setTunnelState('idle')
      return
    }
    setTunnelState((prev) => (prev === 'open' ? prev : 'connecting'))
  }, [tunnel])

  // Process binary data from tunnel
  const processTunnelBuffer = useCallback(
    (buffer: ArrayBuffer) => {
      if (!buffer || buffer.byteLength === 0) {
        return
      }
      let decoder = decoderRef.current
      if (!decoder) {
        decoder = new TextDecoder()
        decoderRef.current = decoder
      }
      const text = decoder.decode(buffer)
      if (!text) {
        return
      }
      onEvent({
        stream: 'ssh.tunnel',
        event: 'stdout',
        sessionId,
        text,
      })
    },
    [onEvent, sessionId]
  )

  // Process control messages from tunnel
  const processTunnelControl = useCallback(
    (payload: string) => {
      if (!payload) {
        return
      }
      let parsed: Record<string, unknown> | null = null
      const trimmed = payload.trimStart()
      if (trimmed.startsWith('{')) {
        try {
          parsed = JSON.parse(trimmed) as Record<string, unknown>
        } catch {
          parsed = null
        }
      }
      if (parsed) {
        const type =
          typeof parsed.type === 'string'
            ? parsed.type.toLowerCase()
            : typeof parsed.event === 'string'
              ? parsed.event.toLowerCase()
              : ''
        const message = typeof parsed.message === 'string' ? parsed.message : undefined
        const connectionId =
          tunnel?.params?.connection_id ?? tunnel?.params?.connectionId ?? undefined
        const eventBase: SshTerminalEvent = {
          stream: 'ssh.tunnel',
          event: type || 'event',
          sessionId,
          connectionId,
          message,
          original: parsed,
        }
        switch (type) {
          case 'ready':
          case 'closed':
          case 'error':
          case 'resize':
            onEvent(eventBase)
            return
          default:
            break
        }
      }
      // Fallback: treat as stdout
      onEvent({
        stream: 'ssh.tunnel',
        event: 'stdout',
        sessionId,
        text: payload,
      })
    },
    [onEvent, sessionId, tunnel?.params?.connectionId, tunnel?.params?.connection_id]
  )

  // Send payload to tunnel
  const sendPayload = useCallback(
    (payload: string | ArrayBuffer): boolean => {
      if (tunnelState !== 'open') {
        return false
      }
      const socket = socketRef.current
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        return false
      }
      try {
        socket.send(payload)
        return true
      } catch {
        return false
      }
    },
    [tunnelState]
  )

  // Send input data to tunnel
  const sendInput = useCallback(
    (data: string): boolean => {
      if (!data) {
        return false
      }
      // Send as binary for reliable terminal input delivery
      const encoder = new TextEncoder()
      const uint8Array = encoder.encode(data)
      return sendPayload(uint8Array.buffer)
    },
    [sendPayload]
  )

  // Send resize command to tunnel
  const sendResize = useCallback(
    (cols: number, rows: number): boolean => {
      if (!cols || !rows || tunnelState !== 'open') {
        return false
      }
      const normalized = normalizeTerminalDimensions(cols, rows)
      const lastSent = lastSentResizeRef.current
      if (lastSent && lastSent.cols === normalized.cols && lastSent.rows === normalized.rows) {
        return true
      }
      const payload = JSON.stringify({
        type: 'resize',
        cols: normalized.cols,
        rows: normalized.rows,
      })
      const ok = sendPayload(payload)
      if (ok) {
        lastSentResizeRef.current = normalized
      }
      return ok
    },
    [sendPayload, tunnelState]
  )

  // Close tunnel connection
  const close = useCallback(() => {
    const socket = socketRef.current
    if (!socket) {
      return
    }
    socketRef.current = null
    try {
      socket.close(1000, 'client closing')
    } catch {
      // ignore failures
    }
    lastSentResizeRef.current = null
  }, [])

  // Tunnel key for dependency tracking
  const tunnelKey = useMemo(() => {
    if (!tunnel) {
      return ''
    }
    const paramsKey = tunnel.params ? JSON.stringify(tunnel.params) : ''
    return `${tunnel.url}|${tunnel.token}|${paramsKey}`
  }, [tunnel])

  // Manage WebSocket connection
  useEffect(() => {
    if (!tunnel || !sessionId) {
      close()
      return
    }

    close()
    setTunnelState('connecting')
    lastSentResizeRef.current = null

    const params: Record<string, string> = {
      ...(tunnel.params ?? {}),
      token: tunnel.token,
    }

    const socketUrl = buildWebSocketUrl(tunnel.url || '/ws', params)
    let disposed = false
    const socket = new WebSocket(socketUrl)
    socket.binaryType = 'arraybuffer'
    socketRef.current = socket

    socket.onopen = () => {
      if (disposed) {
        return
      }
      setTunnelState('open')
    }

    socket.onmessage = (event: MessageEvent) => {
      if (disposed) {
        return
      }
      if (typeof event.data === 'string') {
        processTunnelControl(event.data)
        return
      }
      if (event.data instanceof ArrayBuffer) {
        processTunnelBuffer(event.data)
        return
      }
      if (event.data instanceof Blob) {
        void event.data
          .arrayBuffer()
          .then((buffer) => {
            if (!disposed) {
              processTunnelBuffer(buffer)
            }
          })
          .catch(() => {
            if (!disposed) {
              setTunnelState('failed')
            }
          })
      }
    }

    socket.onerror = () => {
      if (disposed) {
        return
      }
      setTunnelState('failed')
    }

    socket.onclose = (event) => {
      if (disposed) {
        return
      }
      socketRef.current = null
      if (event.wasClean || event.code === 1000) {
        setTunnelState('idle')
        onEvent({
          stream: 'ssh.tunnel',
          event: 'closed',
          sessionId,
          message: event.reason || 'Session closed',
        })
        return
      }
      setTunnelState('failed')
    }

    return () => {
      disposed = true
      if (socketRef.current === socket) {
        socketRef.current = null
      }
      try {
        socket.close(1000, 'client closing')
      } catch {
        // ignore
      }
    }
  }, [close, onEvent, processTunnelBuffer, processTunnelControl, sessionId, tunnel, tunnelKey])

  // Cleanup on unmount
  useEffect(() => close, [close])

  // Clear last sent resize when tunnel closes
  useEffect(() => {
    if (tunnelState !== 'open') {
      lastSentResizeRef.current = null
    }
  }, [tunnelState])

  // Memoize the return object to prevent unnecessary re-renders
  return useMemo(
    () => ({
      tunnelState,
      sendInput,
      sendResize,
      close,
    }),
    [tunnelState, sendInput, sendResize, close]
  )
}
