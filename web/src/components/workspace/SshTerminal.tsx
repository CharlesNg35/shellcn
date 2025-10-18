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
import { clampNumber, isStreamData } from '@/lib/utils/terminal'
import { useSshTerminalStream } from '@/hooks/useSshTerminalStream'
import { useSshTunnel } from '@/hooks/useSshTunnel'
import { useTerminalResize } from '@/hooks/useTerminalResize'
import { getIsTabActive } from '@/hooks/useIsTabActive'
import type { SshTerminalEvent } from '@/types/ssh'
import type { SessionTunnelEntry } from '@/store/ssh-session-tunnel-store'
import { TERMINAL_FONT_SIZE } from '@/constants/terminal'
import { DARK_THEME, resolveTheme } from '@/constants/terminal-themes'
import type { SSHThemeMode } from '@/types/protocol-settings'

import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { WebglAddon } from '@xterm/addon-webgl'

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
  appearance?: TerminalAppearanceOptions
}

export interface SshTerminalHandle {
  focus: () => void
  adjustFontSize: (delta: number) => number
  setFontSize: (fontSize: number) => number
  getFontSize: () => number
  search: (query: string, direction?: 'next' | 'previous') => boolean
  clear: () => void
}

export interface TerminalAppearanceOptions {
  themeMode: SSHThemeMode
  fontFamily: string
  fontSize: number
  scrollbackLimit: number
}

const DEFAULT_APPEARANCE: TerminalAppearanceOptions = {
  themeMode: 'auto',
  fontFamily: 'JetBrains Mono, Menlo, Monaco, Consolas, "Courier New", monospace',
  fontSize: 14,
  scrollbackLimit: 1000,
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
    appearance,
  },
  ref
) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const webglAddonRef = useRef<WebglAddon | null>(null)
  const searchAddonRef = useRef<SearchAddon | null>(null)
  const lastRemoteResizeRef = useRef<{ cols: number; rows: number } | null>(null)
  const suppressResizeRef = useRef(false)

  const [isTerminalReady, setTerminalReady] = useState(false)
  const [status, setStatus] = useState<'connecting' | 'ready' | 'closed' | 'error'>('connecting')
  const [statusMessage, setStatusMessage] = useState<string | undefined>(undefined)
  const [fontSize, setFontSize] = useState<number>(
    appearance?.fontSize ?? DEFAULT_APPEARANCE.fontSize
  )

  // Resolve appearance settings
  const resolvedAppearance = useMemo<TerminalAppearanceOptions>(() => {
    if (!appearance) {
      return { ...DEFAULT_APPEARANCE }
    }
    const themeMode = appearance.themeMode ?? DEFAULT_APPEARANCE.themeMode
    const fontFamily =
      typeof appearance.fontFamily === 'string' && appearance.fontFamily.trim().length > 0
        ? appearance.fontFamily.trim()
        : DEFAULT_APPEARANCE.fontFamily
    const fontSize = clampNumber(
      typeof appearance.fontSize === 'number' ? appearance.fontSize : DEFAULT_APPEARANCE.fontSize,
      TERMINAL_FONT_SIZE.MIN,
      TERMINAL_FONT_SIZE.MAX
    )
    const scrollbackLimit = Math.max(
      200,
      Math.round(
        typeof appearance.scrollbackLimit === 'number'
          ? appearance.scrollbackLimit
          : DEFAULT_APPEARANCE.scrollbackLimit
      )
    )

    return {
      themeMode,
      fontFamily,
      fontSize,
      scrollbackLimit,
    }
  }, [appearance])

  const terminalTheme = useMemo(
    () => resolveTheme(resolvedAppearance.themeMode),
    [resolvedAppearance.themeMode]
  )
  const containerBackground = terminalTheme.background ?? DARK_THEME.background ?? '#1e1e2e'

  // Handle terminal events
  const handleTerminalEvent = useCallback(
    (event: SshTerminalEvent) => {
      if (event.sessionId !== sessionId) {
        return
      }
      if (isStreamData(event)) {
        // Write data directly to terminal (xterm.js handles buffering)
        terminalRef.current?.write(event.text ?? '')
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
              terminalRef.current?.resize(normalizedCols, normalizedRows)
            }
            break
          }
          default:
        }
      }
      onEvent?.(event)
    },
    [sessionId, onEvent]
  )

  // SSH tunnel connection
  const tunnel$ = useSshTunnel({
    tunnel,
    sessionId,
    onEvent: handleTerminalEvent,
  })

  // Determine if we should use realtime stream or tunnel
  const useRealtimeStream = !tunnel || tunnel$.tunnelState === 'failed'
  const isConnected = useRealtimeStream ? false : tunnel$.tunnelState === 'open'

  // Realtime WebSocket stream (fallback)
  const websocket = useSshTerminalStream({
    sessionId,
    onEvent: handleTerminalEvent,
    enabled: Boolean(sessionId) && useRealtimeStream,
  })

  const isWebSocketConnected = useRealtimeStream ? websocket.isConnected : false

  // Handle resize
  const handleResize = useCallback(
    (cols: number, rows: number) => {
      tunnel$.sendResize(cols, rows)
    },
    [tunnel$.sendResize]
  )

  // Terminal resize management
  useTerminalResize({
    containerRef,
    terminalRef,
    fitAddonRef,
    onResize: handleResize,
    enabled: isTerminalReady,
  })

  // Handle tab visibility changes
  useEffect(() => {
    const container = containerRef.current
    const terminal = terminalRef.current
    const fitAddon = fitAddonRef.current

    if (!container || !terminal || !fitAddon || !isTerminalReady) {
      return
    }

    const checkVisibility = () => {
      const isActive = getIsTabActive(container)
      if (isActive) {
        fitAddon.fit()
        terminal.focus()
        if (terminal.cols > 0 && terminal.rows > 0) {
          tunnel$.sendResize(terminal.cols, terminal.rows)
        }
      }
    }

    // Small delay to ensure layout is complete
    const timeoutId = setTimeout(checkVisibility, 100)
    return () => clearTimeout(timeoutId)
  }, [isTerminalReady, tunnel$.sendResize, activeTabId])

  // Initialize terminal
  useEffect(() => {
    let disposed = false
    ;(async () => {
      if (disposed) {
        return
      }

      const terminal = new Terminal({
        allowProposedApi: true,
        convertEol: true,
        cursorBlink: true,
        fontSize: resolvedAppearance.fontSize,
        lineHeight: 1.2,
        fontFamily: resolvedAppearance.fontFamily,
        fontWeight: '400',
        fontWeightBold: '700',
        letterSpacing: 0,
        scrollback: resolvedAppearance.scrollbackLimit,
        theme: terminalTheme,
      })

      const initialFontSize = terminal.options.fontSize ?? resolvedAppearance.fontSize
      setFontSize(initialFontSize)
      onFontSizeChange?.(initialFontSize)

      const fitAddon = new FitAddon()
      terminal.loadAddon(fitAddon)

      const searchAddon = new SearchAddon()
      terminal.loadAddon(searchAddon)
      searchAddonRef.current = searchAddon

      try {
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
      fitAddon.fit()
      terminal.focus()

      terminalRef.current = terminal
      fitAddonRef.current = fitAddon
      setTerminalReady(true)

      // Initial resize will be handled by the tunnel effect
    })()

    return () => {
      disposed = true
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
  }, [onFontSizeChange, resolvedAppearance, terminalTheme])

  // Handle terminal input and resize events
  useEffect(() => {
    const terminal = terminalRef.current
    if (!terminal || !isTerminalReady) {
      return
    }

    // Handle terminal input
    const dataDisposable = terminal.onData((chunk) => {
      tunnel$.sendInput(chunk)
    })

    // Handle terminal resize events
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
      tunnel$.sendResize(normalizedCols, normalizedRows)
    })

    return () => {
      dataDisposable.dispose()
      resizeDisposable.dispose()
    }
  }, [tunnel$.sendInput, tunnel$.sendResize, isTerminalReady])

  // Fit terminal when tunnel opens
  useEffect(() => {
    if (tunnel$.tunnelState !== 'open') {
      return
    }
    const terminal = terminalRef.current
    const fitAddon = fitAddonRef.current
    const container = containerRef.current
    if (!terminal || !fitAddon || !container) {
      return
    }

    const isActive = getIsTabActive(container)
    if (isActive) {
      fitAddon.fit()
      if (terminal.cols > 0 && terminal.rows > 0) {
        tunnel$.sendResize(terminal.cols, terminal.rows)
      }
    }
  }, [tunnel$.tunnelState, tunnel$.sendResize])

  // Imperative handle
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
        const next = Math.max(
          TERMINAL_FONT_SIZE.MIN,
          Math.min(TERMINAL_FONT_SIZE.MAX, (terminal.options.fontSize ?? fontSize) + delta)
        )
        terminal.options.fontSize = next
        setFontSize(next)
        onFontSizeChange?.(next)
        requestAnimationFrame(() => {
          terminal.refresh(0, terminal.rows - 1)
        })
        return next
      },
      setFontSize: (next: number) => {
        const terminal = terminalRef.current
        if (!terminal) {
          return fontSize
        }
        const bounded = Math.max(TERMINAL_FONT_SIZE.MIN, Math.min(TERMINAL_FONT_SIZE.MAX, next))
        terminal.options.fontSize = bounded
        setFontSize(bounded)
        onFontSizeChange?.(bounded)
        requestAnimationFrame(() => {
          terminal.refresh(0, terminal.rows - 1)
        })
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

  // Handle search overlay
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
        className="h-full w-full overflow-hidden p-2"
        style={{ backgroundColor: containerBackground }}
        role="presentation"
        data-testid="ssh-terminal-canvas"
      />

      {/* Only show indicator while connecting, not when live */}
      {status !== 'ready' && (
        <div className="absolute right-3 top-3 flex items-center gap-2 rounded-full bg-slate-900/90 px-3 py-1 text-xs font-medium text-slate-100 shadow-lg backdrop-blur-sm">
          <span
            className={cn('h-2 w-2 rounded-full', {
              'bg-amber-400 animate-pulse': status === 'connecting',
              'bg-rose-500': status === 'error' || (!isConnected && !isWebSocketConnected),
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
          {status === 'connecting' && !isConnected && !isWebSocketConnected && (
            <p className="text-xs text-slate-400">Waiting for realtime connection…</p>
          )}
        </div>
      )}
    </div>
  )
})

export default SshTerminal
