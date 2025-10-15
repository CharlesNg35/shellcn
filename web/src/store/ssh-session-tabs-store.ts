import { create } from 'zustand'
import type { ActiveConnectionSession } from '@/types/connections'

const DEFAULT_TERMINAL_TITLE = 'Terminal'
const DEFAULT_SFTP_TITLE = 'Files'
const DEFAULT_COLUMNS = 1

export type WorkspaceViewType = 'terminal' | 'sftp'

export interface WorkspaceTabMeta {
  badge?: string
  accessMode?: string
  ownerName?: string
  isRecording?: boolean
}

export interface WorkspaceTab {
  id: string
  sessionId: string
  type: WorkspaceViewType
  title: string
  closable: boolean
  meta?: WorkspaceTabMeta
}

export interface SessionWorkspaceState {
  sessionId: string
  connectionId: string
  connectionName?: string
  tabs: WorkspaceTab[]
  activeTabId: string
  layoutColumns: number
  isFullscreen: boolean
  lastFocusedAt: number
}

interface OpenSessionOptions {
  sessionId: string
  connectionId: string
  connectionName?: string
}

interface EnsureTabOptions {
  title?: string
  closable?: boolean
  meta?: WorkspaceTabMeta
}

interface SshWorkspaceStore {
  sessions: Record<string, SessionWorkspaceState>
  orderedSessionIds: string[]
  activeSessionId: string | null
  openSession: (options: OpenSessionOptions) => SessionWorkspaceState
  focusSession: (sessionId: string) => void
  closeSession: (sessionId: string) => void
  ensureTab: (
    sessionId: string,
    type: WorkspaceViewType,
    options?: EnsureTabOptions
  ) => WorkspaceTab
  closeTab: (sessionId: string, tabId: string) => void
  reorderTabs: (sessionId: string, orderedTabIds: string[]) => void
  setActiveTab: (sessionId: string, tabId: string) => void
  updateTabMeta: (sessionId: string, tabId: string, meta: WorkspaceTabMeta) => void
  setLayoutColumns: (sessionId: string, columns: number) => void
  setFullscreen: (sessionId: string, value?: boolean) => void
  reset: () => void
}

function makeTabId(sessionId: string, type: WorkspaceViewType) {
  return `${sessionId}:${type}`
}

function normaliseColumns(value: number) {
  if (!Number.isFinite(value) || value < 1) {
    return DEFAULT_COLUMNS
  }
  if (value > 5) {
    return 5
  }
  return Math.floor(value)
}

const TAB_ORDER_KEY_PREFIX = 'sshWorkspace.tabOrder.'

function getTabOrderKey(sessionId: string) {
  return `${TAB_ORDER_KEY_PREFIX}${sessionId}`
}

function safeWindowStorage() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return undefined
  }
  return window.localStorage
}

function loadTabOrder(sessionId: string): string[] {
  const storage = safeWindowStorage()
  if (!storage) {
    return []
  }
  try {
    const raw = storage.getItem(getTabOrderKey(sessionId))
    if (!raw) {
      return []
    }
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((id) => typeof id === 'string') : []
  } catch {
    return []
  }
}

function persistTabOrder(sessionId: string, order: string[]) {
  const storage = safeWindowStorage()
  if (!storage) {
    return
  }
  try {
    storage.setItem(getTabOrderKey(sessionId), JSON.stringify(order))
  } catch {
    // ignore persistence errors in storage-restricted environments
  }
}

function clearTabOrder(sessionId: string) {
  const storage = safeWindowStorage()
  if (!storage) {
    return
  }
  storage.removeItem(getTabOrderKey(sessionId))
}

function applyTabOrder(tabs: WorkspaceTab[], order: string[]): WorkspaceTab[] {
  if (!order.length) {
    return tabs
  }
  const orderMap = new Map<string, number>()
  order.forEach((id, index) => {
    if (!orderMap.has(id)) {
      orderMap.set(id, index)
    }
  })
  return [...tabs].sort((a, b) => {
    const indexA = orderMap.has(a.id) ? orderMap.get(a.id)! : Number.MAX_SAFE_INTEGER
    const indexB = orderMap.has(b.id) ? orderMap.get(b.id)! : Number.MAX_SAFE_INTEGER
    if (indexA === indexB) {
      return tabs.findIndex((tab) => tab.id === a.id) - tabs.findIndex((tab) => tab.id === b.id)
    }
    return indexA - indexB
  })
}

export const useSshWorkspaceTabsStore = create<SshWorkspaceStore>()((set, get) => ({
  sessions: {},
  orderedSessionIds: [],
  activeSessionId: null,
  openSession: ({ sessionId, connectionId, connectionName }) => {
    const trimmedSession = sessionId.trim()
    const trimmedConnection = connectionId.trim()
    if (!trimmedSession || !trimmedConnection) {
      throw new Error('workspace: sessionId and connectionId are required')
    }
    const state = get()
    const existing = state.sessions[trimmedSession]
    if (existing) {
      return existing
    }

    const terminalTabId = makeTabId(trimmedSession, 'terminal')
    const terminalTab: WorkspaceTab = {
      id: terminalTabId,
      sessionId: trimmedSession,
      type: 'terminal',
      title: DEFAULT_TERMINAL_TITLE,
      closable: false,
      meta: undefined,
    }

    const savedOrder = loadTabOrder(trimmedSession)

    const initialTabs = applyTabOrder([terminalTab], savedOrder)
    const initialActive = initialTabs[0]?.id ?? terminalTabId

    const sessionState: SessionWorkspaceState = {
      sessionId: trimmedSession,
      connectionId: trimmedConnection,
      connectionName,
      tabs: initialTabs,
      activeTabId: initialActive,
      layoutColumns: DEFAULT_COLUMNS,
      isFullscreen: false,
      lastFocusedAt: Date.now(),
    }

    set(() => {
      const ordered = state.orderedSessionIds.filter((id) => id !== trimmedSession)
      ordered.unshift(trimmedSession)
      return {
        sessions: {
          ...state.sessions,
          [trimmedSession]: sessionState,
        },
        orderedSessionIds: ordered,
        activeSessionId: trimmedSession,
      }
    })

    return sessionState
  },
  focusSession: (sessionId) => {
    const trimmed = sessionId.trim()
    if (!trimmed) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      const updatedSession: SessionWorkspaceState = {
        ...session,
        lastFocusedAt: Date.now(),
      }
      const ordered = state.orderedSessionIds.filter((id) => id !== trimmed)
      ordered.unshift(trimmed)
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: updatedSession,
        },
        orderedSessionIds: ordered,
        activeSessionId: trimmed,
      }
    })
  },
  closeSession: (sessionId) => {
    const trimmed = sessionId.trim()
    if (!trimmed) {
      return
    }
    set((state) => {
      if (!state.sessions[trimmed]) {
        return state
      }
      const nextSessions = { ...state.sessions }
      delete nextSessions[trimmed]
      const remainingIds = state.orderedSessionIds.filter((id) => id !== trimmed)
      const nextActive =
        state.activeSessionId === trimmed ? (remainingIds[0] ?? null) : state.activeSessionId
      clearTabOrder(trimmed)
      return {
        ...state,
        sessions: nextSessions,
        orderedSessionIds: remainingIds,
        activeSessionId: nextActive,
      }
    })
  },
  ensureTab: (sessionId, type, options) => {
    const trimmed = sessionId.trim()
    if (!trimmed) {
      throw new Error('workspace: sessionId is required to ensure tab')
    }
    const state = get()
    const session = state.sessions[trimmed]
    if (!session) {
      throw new Error(`workspace: session ${trimmed} has not been opened`)
    }

    const tabId = makeTabId(trimmed, type)
    const existing = session.tabs.find((tab) => tab.id === tabId)
    if (existing) {
      return existing
    }

    const tab: WorkspaceTab = {
      id: tabId,
      sessionId: trimmed,
      type,
      title: options?.title ?? (type === 'sftp' ? DEFAULT_SFTP_TITLE : DEFAULT_TERMINAL_TITLE),
      closable: options?.closable ?? type !== 'terminal',
      meta: options?.meta,
    }

    const nextTabs = applyTabOrder([...session.tabs, tab], loadTabOrder(trimmed))
    persistTabOrder(
      trimmed,
      nextTabs.map((item) => item.id)
    )

    const nextSession: SessionWorkspaceState = {
      ...session,
      tabs: nextTabs,
    }

    set((prev) => ({
      ...prev,
      sessions: {
        ...prev.sessions,
        [trimmed]: nextSession,
      },
    }))

    return tab
  },
  closeTab: (sessionId, tabId) => {
    const trimmed = sessionId.trim()
    if (!trimmed || !tabId) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      if (session.tabs.length <= 1) {
        return state
      }
      if (!session.tabs.some((tab) => tab.id === tabId && tab.closable)) {
        return state
      }
      const remainingTabs = session.tabs.filter((tab) => tab.id !== tabId)
      persistTabOrder(
        trimmed,
        remainingTabs.map((tab) => tab.id)
      )
      const nextActive =
        session.activeTabId === tabId
          ? (remainingTabs[0]?.id ?? session.activeTabId)
          : session.activeTabId
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            tabs: remainingTabs,
            activeTabId: nextActive,
          },
        },
      }
    })
  },
  reorderTabs: (sessionId, orderedTabIds) => {
    const trimmed = sessionId.trim()
    if (!trimmed || !Array.isArray(orderedTabIds) || orderedTabIds.length === 0) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      const nextTabs = applyTabOrder(session.tabs, orderedTabIds)
      persistTabOrder(
        trimmed,
        nextTabs.map((tab) => tab.id)
      )
      const nextActive = nextTabs.some((tab) => tab.id === session.activeTabId)
        ? session.activeTabId
        : (nextTabs[0]?.id ?? session.activeTabId)
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            tabs: nextTabs,
            activeTabId: nextActive,
          },
        },
      }
    })
  },
  setActiveTab: (sessionId, tabId) => {
    const trimmed = sessionId.trim()
    if (!trimmed || !tabId) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      const exists = session.tabs.some((tab) => tab.id === tabId)
      if (!exists) {
        return state
      }
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            activeTabId: tabId,
            lastFocusedAt: Date.now(),
          },
        },
        activeSessionId: trimmed,
      }
    })
  },
  updateTabMeta: (sessionId, tabId, meta) => {
    const trimmed = sessionId.trim()
    if (!trimmed || !tabId) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      const updatedTabs = session.tabs.map((tab) =>
        tab.id === tabId
          ? {
              ...tab,
              meta: {
                ...tab.meta,
                ...meta,
              },
            }
          : tab
      )
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            tabs: updatedTabs,
          },
        },
      }
    })
  },
  setLayoutColumns: (sessionId, columns) => {
    const trimmed = sessionId.trim()
    if (!trimmed) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            layoutColumns: normaliseColumns(columns),
          },
        },
      }
    })
  },
  setFullscreen: (sessionId, value) => {
    const trimmed = sessionId.trim()
    if (!trimmed) {
      return
    }
    set((state) => {
      const session = state.sessions[trimmed]
      if (!session) {
        return state
      }
      const nextValue = typeof value === 'boolean' ? value : !session.isFullscreen
      return {
        ...state,
        sessions: {
          ...state.sessions,
          [trimmed]: {
            ...session,
            isFullscreen: nextValue,
          },
        },
      }
    })
  },
  reset: () =>
    set(() => ({
      sessions: {},
      orderedSessionIds: [],
      activeSessionId: null,
    })),
}))

export function openSessionFromRecord(record: ActiveConnectionSession) {
  const store = useSshWorkspaceTabsStore.getState()
  return store.openSession({
    sessionId: record.id,
    connectionId: record.connection_id,
    connectionName: record.connection_name,
  })
}

export function resetSshWorkspaceTabsStore() {
  useSshWorkspaceTabsStore.getState().reset()
}

export const selectSessionWorkspace =
  (sessionId: string) =>
  (state: SshWorkspaceStore): SessionWorkspaceState | undefined =>
    state.sessions[sessionId]
