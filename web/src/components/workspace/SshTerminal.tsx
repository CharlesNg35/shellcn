import { Loader2 } from 'lucide-react'
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react'

import { cn } from '@/lib/utils/cn'
import { useSshTerminalStream } from '@/hooks/useSshTerminalStream'
import type { SshTerminalEvent } from '@/types/ssh'
import { buildWebSocketUrl } from '@/lib/utils/websocket'
import type { SessionTunnelEntry } from '@/store/ssh-session-tunnel-store'

interface SshTerminalProps {
  sessionId: string
  className?: string
  onEvent?: (event: SshTerminalEvent) => void
  onFontSizeChange?: (fontSize: number) => void
  searchOverlay?: {
    visible: boolean
    query: string
    direction: 'next' | 'previous'
  }
  onSearchResolved?: (result: { matched: boolean }) => void
  tunnel?: SessionTunnelEntry
  activeTabId?: string
}

export interface SshTerminalHandle {
  focus: () => void
  adjustFontSize: (delta: number) => number
  setFontSize: (fontSize: number) => number
  getFontSize: () => number
  search: (query: string, direction?: 'next' | 'previous') => boolean
  clear: () => void
}

type TerminalCtor = typeof import('@xterm/xterm').Terminal
type FitAddonCtor = typeof import('@xterm/addon-fit').FitAddon
type WebglAddonCtor = typeof import('@xterm/addon-webgl').WebglAddon
type SearchAddonCtor = typeof import('@xterm/addon-search').SearchAddon

function isStreamData(event: SshTerminalEvent) {
  const name = event.event.toLowerCase()
  return name === 'stdout' || name === 'stderr'
}

export const SshTerminal = forwardRef<SshTerminalHandle, SshTerminalProps>(function SshTerminal(
  {
    sessionId,
    className,
    onEvent,
    onFontSizeChange,
    searchOverlay,
    onSearchResolved,
    tunnel,
    activeTabId,
  },
  ref
) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<InstanceType<TerminalCtor> | null>(null)
  const fitAddonRef = useRef<InstanceType<FitAddonCtor> | null>(null)
  const webglAddonRef = useRef<InstanceType<WebglAddonCtor> | null>(null)
  const searchAddonRef = useRef<InstanceType<SearchAddonCtor> | null>(null)
  const pendingBufferRef = useRef<string[]>([])
  const flushScheduledRef = useRef(false)
  const frameHandleRef = useRef<number | null>(null)
  const timeoutHandleRef = useRef<number | null>(null)
  const idleHandleRef = useRef<number | null>(null)
  const lastFlushTimeRef = useRef(0)
  const resizeObserverRef = useRef<ResizeObserver | null>(null)
  const directSocketRef = useRef<WebSocket | null>(null)
  const decoderRef = useRef<TextDecoder | null>(null)
  const suppressResizeRef = useRef(false)
  const lastSentResizeRef = useRef<{ cols: number; rows: number } | null>(null)
  const lastRemoteResizeRef = useRef<{ cols: number; rows: number } | null>(null)

  const [isTerminalReady, setTerminalReady] = useState(false)
  const [status, setStatus] = useState<'connecting' | 'ready' | 'closed' | 'error'>('connecting')
  const [statusMessage, setStatusMessage] = useState<string | undefined>(undefined)
  const [fontSize, setFontSize] = useState<number>(14)
  const [tunnelState, setTunnelState] = useState<'idle' | 'connecting' | 'open' | 'failed'>(
    tunnel ? 'connecting' : 'idle'
  )

  useEffect(() => {
    if (!tunnel) {
      setTunnelState('idle')
      return
    }
    setTunnelState((previous) => (previous === 'open' ? previous : 'connecting'))
  }, [tunnel])

  const closeDirectSocket = useCallback(() => {
    const socket = directSocketRef.current
    if (!socket) {
      return
    }
    directSocketRef.current = null
    try {
      socket.close(1000, 'client closing')
    } catch {
      // ignore failures
    }
    lastSentResizeRef.current = null
    lastRemoteResizeRef.current = null
  }, [])

  const cancelScheduledFlush = useCallback(() => {
    if (frameHandleRef.current != null && typeof window !== 'undefined') {
      if (typeof window.cancelAnimationFrame === 'function') {
        window.cancelAnimationFrame(frameHandleRef.current)
      }
      frameHandleRef.current = null
    }
    if (timeoutHandleRef.current != null) {
      clearTimeout(timeoutHandleRef.current)
      timeoutHandleRef.current = null
    }
    if (idleHandleRef.current != null) {
      if (typeof window !== 'undefined') {
        const cancel = (window as WindowWithIdleCallback).cancelIdleCallback
        if (typeof cancel === 'function') {
          cancel(idleHandleRef.current)
        } else {
          clearTimeout(idleHandleRef.current)
        }
      } else {
        clearTimeout(idleHandleRef.current)
      }
      idleHandleRef.current = null
    }
    flushScheduledRef.current = false
  }, [])

  const flushImmediately = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal || pendingBufferRef.current.length === 0) {
      return
    }
    const pending = pendingBufferRef.current.splice(0)
    const combined = pending.join('')
    if (!combined) {
      return
    }
    terminal.write(combined)
    lastFlushTimeRef.current = typeof performance !== 'undefined' ? performance.now() : Date.now()
  }, [])

  const flushBuffered = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal || pendingBufferRef.current.length === 0) {
      flushScheduledRef.current = false
      return
    }
    const combined = pendingBufferRef.current.join('')
    pendingBufferRef.current = []
    const write = () => {
      terminal.write(combined)
      lastFlushTimeRef.current = typeof performance !== 'undefined' ? performance.now() : Date.now()
      flushScheduledRef.current = false
    }
    if (typeof window !== 'undefined') {
      const idle = (window as WindowWithIdleCallback).requestIdleCallback
      if (typeof idle === 'function') {
        idleHandleRef.current = idle(
          () => {
            idleHandleRef.current = null
            write()
          },
          { timeout: 50 }
        )
        return
      }
    }
    write()
  }, [])

  const scheduleFlush = useCallback(() => {
    if (flushScheduledRef.current) {
      return
    }
    flushScheduledRef.current = true
    const budget = 1000 / 120
    const triggerFlush = () => {
      const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
      const elapsed = now - lastFlushTimeRef.current
      if (elapsed >= budget) {
        flushBuffered()
        return
      }
      timeoutHandleRef.current = window.setTimeout(
        () => {
          timeoutHandleRef.current = null
          flushBuffered()
        },
        Math.max(0, budget - elapsed)
      )
    }
    if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
      frameHandleRef.current = window.requestAnimationFrame(() => {
        frameHandleRef.current = null
        triggerFlush()
      })
      return
    }
    timeoutHandleRef.current = window.setTimeout(() => {
      timeoutHandleRef.current = null
      triggerFlush()
    }, budget)
  }, [flushBuffered])

  const flushPending = useCallback(() => {
    flushImmediately()
  }, [flushImmediately])

  const writeChunk = useCallback(
    (chunk?: string) => {
      if (!chunk) {
        return
      }
      pendingBufferRef.current.push(chunk)
      if (terminalRef.current) {
        scheduleFlush()
      }
    },
    [scheduleFlush]
  )

  const handleTerminalEvent = useCallback(
    (event: SshTerminalEvent) => {
      if (event.sessionId !== sessionId) {
        return
      }
      if (isStreamData(event)) {
        writeChunk(event.text)
      } else {
        switch (event.event.toLowerCase()) {
          case 'ready':
            setStatus('ready')
            setStatusMessage(undefined)
            break
          case 'closed':
            setStatus('closed')
            setStatusMessage(event.message ?? 'Session closed')
            break
          case 'error':
            setStatus('error')
            setStatusMessage(event.message ?? 'An error occurred')
            break
          case 'resize': {
            const cols = Number(event.original?.cols ?? 0)
            const rows = Number(event.original?.rows ?? 0)
            if (cols > 0 && rows > 0) {
              const normalizedCols = Math.max(1, Math.floor(cols))
              const normalizedRows = Math.max(1, Math.floor(rows))
              suppressResizeRef.current = true
              lastRemoteResizeRef.current = { cols: normalizedCols, rows: normalizedRows }
              lastSentResizeRef.current = { cols: normalizedCols, rows: normalizedRows }
              terminalRef.current?.resize(normalizedCols, normalizedRows)
            }
            break
          }
          default:
        }
      }
      onEvent?.(event)
    },
    [sessionId, writeChunk, onEvent]
  )

  const sendTunnelPayload = useCallback(
    (payload: string | ArrayBuffer) => {
      if (tunnelState !== 'open') {
        return false
      }
      const socket = directSocketRef.current
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

  const sendTunnelInput = useCallback(
    (data: string) => {
      if (!data) {
        return false
      }
      // Send as binary for reliable terminal input delivery
      const encoder = new TextEncoder()
      const uint8Array = encoder.encode(data)
      return sendTunnelPayload(uint8Array.buffer)
    },
    [sendTunnelPayload]
  )

  const sendTunnelResize = useCallback(
    (cols: number, rows: number) => {
      if (!cols || !rows || tunnelState !== 'open') {
        return false
      }
      const normalizedCols = Math.max(1, Math.floor(cols))
      const normalizedRows = Math.max(1, Math.floor(rows))
      const lastSent = lastSentResizeRef.current
      if (lastSent && lastSent.cols === normalizedCols && lastSent.rows === normalizedRows) {
        return true
      }
      const payload = JSON.stringify({ type: 'resize', cols: normalizedCols, rows: normalizedRows })
      const ok = sendTunnelPayload(payload)
      if (ok) {
        lastSentResizeRef.current = { cols: normalizedCols, rows: normalizedRows }
      }
      return ok
    },
    [sendTunnelPayload, tunnelState]
  )

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
      handleTerminalEvent({
        stream: 'ssh.tunnel',
        event: 'stdout',
        sessionId,
        text,
      })
    },
    [handleTerminalEvent, sessionId]
  )

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
            handleTerminalEvent(eventBase)
            return
          case 'closed':
            handleTerminalEvent(eventBase)
            return
          case 'error':
            handleTerminalEvent(eventBase)
            setStatus('error')
            setStatusMessage(message ?? 'Tunnel error')
            return
          case 'resize': {
            const colsValue = Number(parsed.cols)
            const rowsValue = Number(parsed.rows)
            if (Number.isFinite(colsValue) && Number.isFinite(rowsValue)) {
              lastRemoteResizeRef.current = {
                cols: Math.max(1, Math.floor(colsValue)),
                rows: Math.max(1, Math.floor(rowsValue)),
              }
            }
            handleTerminalEvent(eventBase)
            return
          }
          default:
            break
        }
      }
      handleTerminalEvent({
        stream: 'ssh.tunnel',
        event: 'stdout',
        sessionId,
        text: payload,
      })
    },
    [handleTerminalEvent, sessionId, tunnel?.params?.connectionId, tunnel?.params?.connection_id]
  )

  const tunnelKey = useMemo(() => {
    if (!tunnel) {
      return ''
    }
    const paramsKey = tunnel.params ? JSON.stringify(tunnel.params) : ''
    return `${tunnel.url}|${tunnel.token}|${paramsKey}`
  }, [tunnel])

  useEffect(() => {
    if (!tunnel || !sessionId) {
      closeDirectSocket()
      return
    }

    closeDirectSocket()
    setStatus('connecting')
    setStatusMessage(undefined)
    setTunnelState('connecting')
    lastSentResizeRef.current = null
    lastRemoteResizeRef.current = null
    suppressResizeRef.current = false

    const params: Record<string, string> = {
      ...(tunnel.params ?? {}),
      token: tunnel.token,
    }

    const socketUrl = buildWebSocketUrl(tunnel.url || '/ws', params)
    let disposed = false
    const socket = new WebSocket(socketUrl)
    socket.binaryType = 'arraybuffer'
    directSocketRef.current = socket

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
              setStatus('error')
              setStatusMessage('Failed to read tunnel payload')
            }
          })
      }
    }

    socket.onerror = () => {
      if (disposed) {
        return
      }
      setTunnelState('failed')
      setStatus('error')
      setStatusMessage('Unable to establish tunnel connection')
    }

    socket.onclose = (event) => {
      if (disposed) {
        return
      }
      directSocketRef.current = null
      if (event.wasClean || event.code === 1000) {
        setTunnelState('idle')
        handleTerminalEvent({
          stream: 'ssh.tunnel',
          event: 'closed',
          sessionId,
          message: event.reason || 'Session closed',
        })
        return
      }
      setTunnelState('failed')
      setStatus('error')
      setStatusMessage(event.reason || 'Tunnel connection closed unexpectedly')
    }

    return () => {
      disposed = true
      if (directSocketRef.current === socket) {
        directSocketRef.current = null
      }
      try {
        socket.close(1000, 'client closing')
      } catch {
        // ignore
      }
    }
  }, [
    closeDirectSocket,
    handleTerminalEvent,
    processTunnelBuffer,
    processTunnelControl,
    sessionId,
    tunnel,
    tunnelKey,
  ])

  useEffect(() => closeDirectSocket, [closeDirectSocket])

  // Handle visibility changes (when tab becomes active)
  useEffect(() => {
    const container = containerRef.current
    const terminal = terminalRef.current
    const fitAddon = fitAddonRef.current

    if (!container || !terminal || !fitAddon) {
      return
    }

    // Check if container is visible (check parent TabsContent data-state)
    const checkVisibility = () => {
      if (!isTerminalReady) {
        return
      }

      // Find parent TabsContent element
      let parent = container.parentElement
      while (parent && parent.getAttribute('data-radix-tabs-content') === null) {
        parent = parent.parentElement
      }

      // Check if tab is active (data-state="active")
      const isActive = parent?.getAttribute('data-state') === 'active'

      if (isActive) {
        fitAddon.fit()
        terminal.focus()
        if (terminal.cols > 0 && terminal.rows > 0) {
          sendTunnelResize(terminal.cols, terminal.rows)
        }
      }
    }

    // Small delay to ensure layout is complete
    const timeoutId = setTimeout(checkVisibility, 100)

    return () => clearTimeout(timeoutId)
  }, [isTerminalReady, sendTunnelResize, activeTabId])

  useEffect(() => {
    let disposed = false
    ;(async () => {
      const [{ Terminal }, { FitAddon }, { SearchAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit'),
        import('@xterm/addon-search'),
      ])
      if (disposed) {
        return
      }

      const terminal = new Terminal({
        allowProposedApi: true,
        convertEol: true,
        cursorBlink: true,
        fontSize: 13,
        lineHeight: 1.2,
        fontFamily: 'JetBrains Mono, Menlo, Monaco, Consolas, "Courier New", monospace',
        fontWeight: '400',
        fontWeightBold: '700',
        letterSpacing: 0,
        theme: {
          background: '#1e1e2e',
          foreground: '#cdd6f4',
          cursor: '#f5e0dc',
          cursorAccent: '#1e1e2e',
          selectionBackground: '#45475a',
          selectionForeground: '#cdd6f4',
          black: '#45475a',
          red: '#f38ba8',
          green: '#a6e3a1',
          yellow: '#f9e2af',
          blue: '#89b4fa',
          magenta: '#f5c2e7',
          cyan: '#94e2d5',
          white: '#bac2de',
          brightBlack: '#585b70',
          brightRed: '#f38ba8',
          brightGreen: '#a6e3a1',
          brightYellow: '#f9e2af',
          brightBlue: '#89b4fa',
          brightMagenta: '#f5c2e7',
          brightCyan: '#94e2d5',
          brightWhite: '#a6adc8',
        },
      })
      const initialFontSize = terminal.options.fontSize ?? 13
      setFontSize(initialFontSize)
      onFontSizeChange?.(initialFontSize)

      const fitAddon = new FitAddon()
      terminal.loadAddon(fitAddon)

      const searchAddon = new SearchAddon()
      terminal.loadAddon(searchAddon)
      searchAddonRef.current = searchAddon

      try {
        const { WebglAddon } = await import('@xterm/addon-webgl')
        if (!disposed) {
          const webglAddon = new WebglAddon()
          terminal.loadAddon(webglAddon)
          webglAddonRef.current = webglAddon
        }
      } catch (error) {
        if (import.meta.env.DEV) {
          console.warn('Unable to load xterm WebGL addon', error)
        }
      }

      const host = containerRef.current
      if (!host || disposed) {
        terminal.dispose()
        fitAddon.dispose()
        return
      }

      terminal.open(host)

      // Always fit on mount - the visibility effect will handle re-fitting when tab becomes visible
      fitAddon.fit()
      terminal.focus()
      if (terminal.cols > 0 && terminal.rows > 0) {
        sendTunnelResize(terminal.cols, terminal.rows)
      }

      terminalRef.current = terminal
      fitAddonRef.current = fitAddon
      setTerminalReady(true)
      flushPending()

      const dataDisposable = terminal.onData((chunk) => {
        sendTunnelInput(chunk)
      })

      const resizeDisposable = terminal.onResize(({ cols, rows }) => {
        const normalizedCols = Math.max(1, Math.floor(cols))
        const normalizedRows = Math.max(1, Math.floor(rows))
        if (!Number.isFinite(normalizedCols) || !Number.isFinite(normalizedRows)) {
          return
        }
        if (suppressResizeRef.current) {
          suppressResizeRef.current = false
          lastRemoteResizeRef.current = { cols: normalizedCols, rows: normalizedRows }
          return
        }
        const remote = lastRemoteResizeRef.current
        if (remote && remote.cols === normalizedCols && remote.rows === normalizedRows) {
          lastRemoteResizeRef.current = null
          return
        }
        sendTunnelResize(normalizedCols, normalizedRows)
      })

      const observer = new ResizeObserver(() => {
        // Only resize if the tab is active
        let parent = host.parentElement
        while (parent && parent.getAttribute('data-radix-tabs-content') === null) {
          parent = parent.parentElement
        }
        const isActive = parent?.getAttribute('data-state') === 'active'

        if (!isActive) {
          return
        }

        fitAddon.fit()
        if (terminal.cols > 0 && terminal.rows > 0) {
          sendTunnelResize(terminal.cols, terminal.rows)
        }
      })
      observer.observe(host)
      resizeObserverRef.current = observer

      return () => {
        dataDisposable.dispose()
        resizeDisposable.dispose()
      }
    })()

    return () => {
      disposed = true
      cancelScheduledFlush()
      resizeObserverRef.current?.disconnect()
      resizeObserverRef.current = null
      webglAddonRef.current?.dispose()
      webglAddonRef.current = null
      searchAddonRef.current?.dispose()
      searchAddonRef.current = null
      fitAddonRef.current?.dispose()
      fitAddonRef.current = null
      terminalRef.current?.dispose()
      terminalRef.current = null
      setTerminalReady(false)
    }
  }, [cancelScheduledFlush, flushPending, onFontSizeChange, sendTunnelInput, sendTunnelResize])

  useImperativeHandle(
    ref,
    () => ({
      focus: () => {
        terminalRef.current?.focus()
      },
      adjustFontSize: (delta: number) => {
        const terminal = terminalRef.current
        if (!terminal) {
          return fontSize
        }
        const next = Math.max(8, Math.min(32, (terminal.options.fontSize ?? fontSize) + delta))
        terminal.options.fontSize = next
        setFontSize(next)
        onFontSizeChange?.(next)
        return next
      },
      setFontSize: (next: number) => {
        const terminal = terminalRef.current
        if (!terminal) {
          return fontSize
        }
        const bounded = Math.max(8, Math.min(32, next))
        terminal.options.fontSize = bounded
        setFontSize(bounded)
        onFontSizeChange?.(bounded)
        return bounded
      },
      getFontSize: () => terminalRef.current?.options.fontSize ?? fontSize,
      search: (query: string, direction: 'next' | 'previous' = 'next') => {
        const addon = searchAddonRef.current
        if (!addon || !query) {
          return false
        }
        const matched =
          direction === 'previous'
            ? addon.findPrevious(query, { incremental: false })
            : addon.findNext(query, { incremental: false })
        onSearchResolved?.({ matched })
        return matched
      },
      clear: () => {
        terminalRef.current?.clear()
      },
    }),
    [fontSize, onFontSizeChange, onSearchResolved]
  )

  useEffect(() => {
    if (!searchOverlay?.visible || !searchOverlay.query) {
      return
    }
    const addon = searchAddonRef.current
    if (!addon) {
      return
    }
    const matched =
      searchOverlay.direction === 'previous'
        ? addon.findPrevious(searchOverlay.query, { incremental: false })
        : addon.findNext(searchOverlay.query, { incremental: false })
    onSearchResolved?.({ matched })
  }, [searchOverlay, onSearchResolved])

  useEffect(() => {
    if (tunnelState !== 'open') {
      return
    }
    const terminal = terminalRef.current
    const fitAddon = fitAddonRef.current
    const container = containerRef.current
    if (!terminal || !fitAddon || !container) {
      return
    }

    // Only fit and resize if the tab is active
    let parent = container.parentElement
    while (parent && parent.getAttribute('data-radix-tabs-content') === null) {
      parent = parent.parentElement
    }
    const isActive = parent?.getAttribute('data-state') === 'active'

    if (isActive) {
      fitAddon.fit()
      if (terminal.cols > 0 && terminal.rows > 0) {
        sendTunnelResize(terminal.cols, terminal.rows)
      }
    }
  }, [sendTunnelResize, tunnelState])

  const useRealtimeStream = !tunnel || tunnelState === 'failed'
  const websocket = useSshTerminalStream({
    sessionId,
    onEvent: handleTerminalEvent,
    enabled: Boolean(sessionId) && useRealtimeStream,
  })

  const isConnected = useRealtimeStream ? websocket.isConnected : tunnelState === 'open'

  useEffect(() => {
    if (tunnelState !== 'open') {
      lastSentResizeRef.current = null
    }
  }, [tunnelState])

  const statusLabel = useMemo(() => {
    switch (status) {
      case 'ready':
        return 'Live'
      case 'closed':
        return 'Closed'
      case 'error':
        return 'Error'
      default:
        return 'Connecting'
    }
  }, [status])

  const shouldShowOverlay = !isTerminalReady || status === 'error' || status === 'closed'

  return (
    <div
      className={cn(
        'relative h-full w-full overflow-hidden rounded-lg border border-border',
        className
      )}
    >
      <div
        ref={containerRef}
        className="h-full w-full p-2"
        style={{ backgroundColor: '#1e1e2e' }}
        role="presentation"
        data-testid="ssh-terminal-canvas"
      />

      {/* Only show indicator while connecting, not when live */}
      {status !== 'ready' && (
        <div className="absolute right-3 top-3 flex items-center gap-2 rounded-full bg-slate-900/90 px-3 py-1 text-xs font-medium text-slate-100 shadow-lg backdrop-blur-sm">
          <span
            className={cn('h-2 w-2 rounded-full', {
              'bg-amber-400 animate-pulse': status === 'connecting',
              'bg-rose-500': status === 'error' || !isConnected,
              'bg-slate-500': status === 'closed',
            })}
          />
          <span>{statusLabel}</span>
        </div>
      )}

      {shouldShowOverlay && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-2 bg-slate-950/80 text-sm text-slate-200">
          {status === 'connecting' && (
            <Loader2 className="h-5 w-5 animate-spin text-cyan-400" aria-hidden />
          )}
          <p className="font-medium">
            {status === 'connecting'
              ? 'Establishing terminal session…'
              : status === 'error'
                ? (statusMessage ?? 'The session encountered an error.')
                : (statusMessage ?? 'The session has ended.')}
          </p>
          {status === 'connecting' && !isConnected && (
            <p className="text-xs text-slate-400">Waiting for realtime connection…</p>
          )}
        </div>
      )}
    </div>
  )
})

export default SshTerminal
