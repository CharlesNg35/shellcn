import { useCallback, useEffect, useMemo, useState } from 'react'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import type { ActiveConnectionSession } from '@/types/connections'

interface UseCommandPaletteStateParams {
  tabs: WorkspaceTab[]
  activeTabId: string
  sessionId: string
  orderedSessionIds: string[]
  activeSessions: ActiveConnectionSession[]
  setActiveTab: (sessionId: string, tabId: string) => void
  navigate: (path: string) => void
}

interface CommandPaletteEntry {
  id: string
  label: string
  isActive: boolean
  onSelect: () => void
}

interface CommandPaletteSessionEntry {
  id: string
  label: string
  onNavigate: () => void
}

interface UseCommandPaletteStateResult {
  isOpen: boolean
  open: () => void
  close: () => void
  toggle: () => void
  paletteTabs: CommandPaletteEntry[]
  paletteSessions: CommandPaletteSessionEntry[]
}

export function useCommandPaletteState({
  tabs,
  activeTabId,
  sessionId,
  orderedSessionIds,
  activeSessions,
  setActiveTab,
  navigate,
}: UseCommandPaletteStateParams): UseCommandPaletteStateResult {
  const [isOpen, setIsOpen] = useState(false)

  const open = useCallback(() => setIsOpen(true), [])
  const close = useCallback(() => setIsOpen(false), [])
  const toggle = useCallback(() => setIsOpen((previous) => !previous), [])

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        toggle()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [toggle])

  const paletteTabs = useMemo<CommandPaletteEntry[]>(
    () =>
      tabs.map((tab) => ({
        id: tab.id,
        label: tab.title,
        isActive: tab.id === activeTabId,
        onSelect: () => setActiveTab(sessionId, tab.id),
      })),
    [activeTabId, sessionId, setActiveTab, tabs]
  )

  const paletteSessions = useMemo<CommandPaletteSessionEntry[]>(() => {
    if (!sessionId) {
      return []
    }
    return orderedSessionIds
      .filter((id) => id !== sessionId)
      .map((id) => {
        const record = activeSessions.find((entry) => entry.id === id)
        const label = record?.connection_name ?? record?.connection_id ?? id
        return {
          id,
          label,
          onNavigate: () => navigate(`/active-sessions/${id}`),
        }
      })
  }, [activeSessions, navigate, orderedSessionIds, sessionId])

  return {
    isOpen,
    open,
    close,
    toggle,
    paletteTabs,
    paletteSessions,
  }
}
