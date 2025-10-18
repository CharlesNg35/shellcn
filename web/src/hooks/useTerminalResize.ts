/**
 * Hook for managing terminal resize behavior
 */
import { useEffect, useRef, type RefObject } from 'react'
import type { Terminal } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'
import { getIsTabActive } from './useIsTabActive'

interface UseTerminalResizeOptions {
  containerRef: RefObject<HTMLElement | null>
  terminalRef: RefObject<Terminal | null>
  fitAddonRef: RefObject<FitAddon | null>
  onResize?: (cols: number, rows: number) => void
  enabled: boolean
}

/**
 * Manages terminal resize behavior including:
 * - ResizeObserver for container size changes
 * - Tab visibility detection
 * - Debounced resize operations
 */
export function useTerminalResize({
  containerRef,
  terminalRef,
  fitAddonRef,
  onResize,
  enabled,
}: UseTerminalResizeOptions): void {
  const resizeObserverRef = useRef<ResizeObserver | null>(null)
  const onResizeRef = useRef(onResize)

  // Keep onResize ref up to date
  useEffect(() => {
    onResizeRef.current = onResize
  }, [onResize])

  // Setup ResizeObserver
  useEffect(() => {
    if (!enabled) {
      return
    }

    const container = containerRef.current
    const terminal = terminalRef.current
    const fitAddon = fitAddonRef.current

    if (!container || !terminal || !fitAddon) {
      return
    }

    const observer = new ResizeObserver(() => {
      // Only resize if the tab is active
      const isActive = getIsTabActive(container)
      if (!isActive) {
        return
      }

      fitAddon.fit()
      if (terminal.cols > 0 && terminal.rows > 0) {
        onResizeRef.current?.(terminal.cols, terminal.rows)
      }
    })

    observer.observe(container)
    resizeObserverRef.current = observer

    return () => {
      observer.disconnect()
      resizeObserverRef.current = null
    }
  }, [containerRef, terminalRef, fitAddonRef, enabled])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      resizeObserverRef.current?.disconnect()
      resizeObserverRef.current = null
    }
  }, [])
}
