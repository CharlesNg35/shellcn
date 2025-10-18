import { useCallback, useRef, useState } from 'react'
import type { SshTerminalHandle } from '@/components/workspace/SshTerminal'
import { TERMINAL_FONT_SIZE } from '@/constants/terminal'

interface UseWorkspaceTelemetryParams {
  terminalRef: React.RefObject<SshTerminalHandle | null>
  logEvent: (action: string, details?: Record<string, unknown>) => void
}

export interface WorkspaceTelemetryControls {
  fontSize: number
  setFontSize: (value: number) => void
  handleTerminalEvent: () => void
  latencyMs: number | null
  lastActivityAt: Date | null
  zoomIn: () => void
  zoomOut: () => void
  zoomReset: () => void
}

export function useWorkspaceTelemetry({
  terminalRef,
  logEvent,
}: UseWorkspaceTelemetryParams): WorkspaceTelemetryControls {
  const lastEventTimestampRef = useRef<number | null>(null)
  const [latencyMs, setLatencyMs] = useState<number | null>(null)
  const [lastActivityAt, setLastActivityAt] = useState<Date | null>(null)
  const [fontSize, setFontSize] = useState<number>(TERMINAL_FONT_SIZE.DEFAULT)

  const handleTerminalEvent = useCallback(() => {
    const now = performance.now()
    if (lastEventTimestampRef.current != null) {
      setLatencyMs(Math.max(0, now - lastEventTimestampRef.current))
    }
    lastEventTimestampRef.current = now
    setLastActivityAt(new Date())
  }, [])

  const applyFontSizeDelta = useCallback(
    (delta: number, direction: 'in' | 'out') => {
      const next = terminalRef.current?.adjustFontSize(delta)
      if (next !== undefined) {
        setFontSize(next)
        logEvent('terminal.zoom', { direction, fontSize: next })
      }
    },
    [logEvent, terminalRef]
  )

  const zoomIn = useCallback(() => {
    applyFontSizeDelta(1, 'in')
  }, [applyFontSizeDelta])

  const zoomOut = useCallback(() => {
    applyFontSizeDelta(-1, 'out')
  }, [applyFontSizeDelta])

  const zoomReset = useCallback(() => {
    const next = terminalRef.current?.setFontSize(TERMINAL_FONT_SIZE.DEFAULT)
    if (next !== undefined) {
      setFontSize(next)
      logEvent('terminal.zoom.reset', { fontSize: next })
    }
  }, [logEvent, terminalRef])

  const handleFontSizeChange = useCallback((value: number) => {
    setFontSize(value)
  }, [])

  return {
    fontSize,
    setFontSize: handleFontSizeChange,
    handleTerminalEvent,
    latencyMs,
    lastActivityAt,
    zoomIn,
    zoomOut,
    zoomReset,
  }
}
