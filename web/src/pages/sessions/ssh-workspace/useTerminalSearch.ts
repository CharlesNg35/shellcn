import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

interface UseTerminalSearchParams {
  performSearch: (query: string, direction: 'next' | 'previous') => boolean
  logEvent: (action: string, details?: Record<string, unknown>) => void
  sessionId?: string
}

export interface TerminalSearchControls {
  overlay: {
    visible: boolean
    query: string
    direction: 'next' | 'previous'
  }
  isOpen: boolean
  query: string
  direction: 'next' | 'previous'
  matched: boolean
  toggle: () => void
  onQueryChange: (value: string) => void
  onDirectionChange: (direction: 'next' | 'previous') => void
  onSubmit: () => void
  onResolved: (matched: boolean) => void
  inputRef: React.RefObject<HTMLInputElement | null>
}

export function useTerminalSearch({
  performSearch,
  logEvent,
  sessionId,
}: UseTerminalSearchParams): TerminalSearchControls {
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [direction, setDirection] = useState<'next' | 'previous'>('next')
  const [matched, setMatched] = useState(true)

  useEffect(() => {
    if (!isOpen) {
      setMatched(true)
      return
    }
    const handle = window.setTimeout(() => {
      searchInputRef.current?.focus()
    }, 10)
    return () => window.clearTimeout(handle)
  }, [isOpen])

  const toggle = useCallback(() => {
    setIsOpen((previous) => {
      if (!previous) {
        logEvent('terminal.search.open', { sessionId })
      }
      return !previous
    })
  }, [logEvent, sessionId])

  const onQueryChange = useCallback((value: string) => {
    setQuery(value)
    setMatched(true)
  }, [])

  const onDirectionChange = useCallback((nextDirection: 'next' | 'previous') => {
    setDirection(nextDirection)
  }, [])

  const onSubmit = useCallback(() => {
    if (!query) {
      return
    }
    const matchedResult = performSearch(query, direction)
    setMatched(Boolean(matchedResult))
    logEvent('terminal.search', {
      sessionId,
      queryLength: query.length,
      direction,
      matched: Boolean(matchedResult),
    })
  }, [direction, logEvent, performSearch, query, sessionId])

  const overlay = useMemo(
    () => ({
      visible: isOpen,
      query,
      direction,
    }),
    [direction, isOpen, query]
  )

  return {
    overlay,
    isOpen,
    query,
    direction,
    matched,
    toggle,
    onQueryChange,
    onDirectionChange,
    onSubmit,
    onResolved: setMatched,
    inputRef: searchInputRef,
  }
}
