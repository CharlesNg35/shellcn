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
  { sessionId, className, onEvent, onFontSizeChange, searchOverlay, onSearchResolved },
  ref
) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<InstanceType<TerminalCtor> | null>(null)
  const fitAddonRef = useRef<InstanceType<FitAddonCtor> | null>(null)
  const webglAddonRef = useRef<InstanceType<WebglAddonCtor> | null>(null)
  const searchAddonRef = useRef<InstanceType<SearchAddonCtor> | null>(null)
  const pendingBufferRef = useRef<string[]>([])
  const resizeObserverRef = useRef<ResizeObserver | null>(null)

  const [isTerminalReady, setTerminalReady] = useState(false)
  const [status, setStatus] = useState<'connecting' | 'ready' | 'closed' | 'error'>('connecting')
  const [statusMessage, setStatusMessage] = useState<string | undefined>(undefined)
  const [fontSize, setFontSize] = useState<number>(14)

  const flushPending = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal) {
      return
    }
    const pending = pendingBufferRef.current.splice(0)
    pending.forEach((chunk) => {
      terminal.write(chunk)
    })
  }, [])

  const writeChunk = useCallback((chunk?: string) => {
    if (!chunk) {
      return
    }
    const terminal = terminalRef.current
    if (!terminal) {
      pendingBufferRef.current.push(chunk)
      return
    }
    terminal.write(chunk)
  }, [])

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
              terminalRef.current?.resize(cols, rows)
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

  const websocket = useSshTerminalStream({
    sessionId,
    onEvent: handleTerminalEvent,
    enabled: Boolean(sessionId),
  })

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
        fontFamily:
          'var(--font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace)',
        theme: {
          background: '#0f172a',
          cursor: '#22d3ee',
        },
      })
      const initialFontSize = terminal.options.fontSize ?? 14
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
      fitAddon.fit()
      terminal.focus()

      terminalRef.current = terminal
      fitAddonRef.current = fitAddon
      setTerminalReady(true)
      flushPending()

      const observer = new ResizeObserver(() => {
        fitAddon.fit()
      })
      observer.observe(host)
      resizeObserverRef.current = observer
    })()

    return () => {
      disposed = true
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
  }, [flushPending, onFontSizeChange])

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

  const isConnected = websocket.isConnected

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
        className="h-full w-full bg-slate-950/95"
        role="presentation"
        data-testid="ssh-terminal-canvas"
      />

      <div className="absolute left-3 top-3 flex items-center gap-2 rounded-full bg-slate-900/80 px-3 py-1 text-xs font-medium text-slate-100 shadow-sm">
        <span
          className={cn('h-2 w-2 rounded-full', {
            'bg-emerald-400 animate-pulse': status === 'ready' && isConnected,
            'bg-amber-400 animate-pulse': status === 'connecting',
            'bg-rose-500': status === 'error' || !isConnected,
            'bg-slate-500': status === 'closed',
          })}
        />
        <span>{statusLabel}</span>
      </div>

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
