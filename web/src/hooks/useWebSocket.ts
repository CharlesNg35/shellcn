import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

export interface UseWebSocketOptions<TMessage = unknown> {
  onMessage?: (message: TMessage) => void
  onConnect?: () => void
  onDisconnect?: (event?: CloseEvent) => void
  onError?: (event: Event) => void
  autoReconnect?: boolean
  reconnectInterval?: number
  enabled?: boolean
  parseJson?: boolean
}

export interface UseWebSocketResult<TMessage = unknown> {
  isConnected: boolean
  send: (data: unknown) => void
  close: () => void
  lastMessage: TMessage | null
}

export function useWebSocket<TMessage = unknown>(
  url: string,
  options: UseWebSocketOptions<TMessage> = {}
): UseWebSocketResult<TMessage> {
  const {
    onMessage,
    onConnect,
    onDisconnect,
    onError,
    autoReconnect = true,
    reconnectInterval = 3_000,
    enabled = true,
    parseJson = true,
  } = options

  const [isConnected, setIsConnected] = useState(false)
  const [lastMessage, setLastMessage] = useState<TMessage | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const cleanupConnection = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setIsConnected(false)
  }, [])

  useEffect(() => {
    if (!enabled || !url || typeof window === 'undefined') {
      return undefined
    }

    const connect = () => {
      try {
        const socket = new WebSocket(url)

        socket.onopen = () => {
          setIsConnected(true)
          onConnect?.()
        }

        socket.onmessage = (event) => {
          let payload: unknown = event.data
          if (parseJson && typeof event.data === 'string') {
            try {
              payload = JSON.parse(event.data)
            } catch (error) {
              console.error('Failed to parse WebSocket message', error)
            }
          }
          setLastMessage(payload as TMessage)
          onMessage?.(payload as TMessage)
        }

        socket.onerror = (event) => {
          onError?.(event)
        }

        socket.onclose = (event) => {
          setIsConnected(false)
          onDisconnect?.(event)

          if (autoReconnect) {
            reconnectTimerRef.current = setTimeout(connect, reconnectInterval)
          }
        }

        wsRef.current = socket
      } catch (error) {
        console.error('Unable to establish WebSocket connection', error)
        if (autoReconnect) {
          reconnectTimerRef.current = setTimeout(connect, reconnectInterval)
        }
      }
    }

    connect()

    return () => {
      clearReconnectTimer()
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.close()
      }
      cleanupConnection()
    }
  }, [
    autoReconnect,
    cleanupConnection,
    clearReconnectTimer,
    enabled,
    onConnect,
    onDisconnect,
    onError,
    onMessage,
    parseJson,
    reconnectInterval,
    url,
  ])

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const payload = typeof data === 'string' ? data : JSON.stringify(data)
      wsRef.current.send(payload)
    } else {
      console.warn('WebSocket is not connected')
    }
  }, [])

  const close = useCallback(() => {
    clearReconnectTimer()
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.close()
    }
  }, [clearReconnectTimer])

  return useMemo(
    () => ({
      isConnected,
      send,
      close,
      lastMessage,
    }),
    [close, isConnected, lastMessage, send]
  )
}
