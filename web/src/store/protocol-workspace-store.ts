import { create } from 'zustand'
import type { SftpListResult } from '@/types/sftp'

const BASE_BROWSER_TITLE = 'Files'

type WorkspaceTabType = 'browser' | 'editor'

export interface WorkspaceTab {
  id: string
  type: WorkspaceTabType
  title: string
  path?: string
  dirty?: boolean
}

export interface WorkspaceSessionState {
  sessionId: string
  browserPath: string
  homeDirectory?: string
  showHidden: boolean
  tabs: WorkspaceTab[]
  activeTabId: string
  transfers: Record<string, TransferItem>
  transferOrder: string[]
  directoryCache: Record<string, DirectoryCacheEntry>
  directoryCacheOrder: string[]
}

interface DirectoryCacheEntry {
  data: SftpListResult
  fetchedAt: number
}

export interface TransferItem {
  id: string
  remoteId?: string
  name: string
  path: string
  direction: string
  size: number
  uploaded: number
  status: 'pending' | 'uploading' | 'completed' | 'failed'
  startedAt: Date
  completedAt?: Date
  errorMessage?: string
  totalBytes?: number
  userId?: string
  userName?: string
}

interface WorkspaceStore {
  sessions: Record<string, WorkspaceSessionState>
  ensureSession: (sessionId: string) => WorkspaceSessionState
  setBrowserPath: (sessionId: string, path: string) => void
  setHomeDirectory: (sessionId: string, path: string) => void
  setShowHidden: (sessionId: string, show: boolean) => void
  setActiveTab: (sessionId: string, tabId: string) => void
  openEditor: (sessionId: string, path: string, title?: string) => void
  closeTab: (sessionId: string, tabId: string) => void
  setTabDirty: (sessionId: string, tabId: string, dirty: boolean) => void
  upsertTransfer: (sessionId: string, transfer: TransferItem) => void
  updateTransfer: (
    sessionId: string,
    transferId: string,
    updater: (transfer: TransferItem) => TransferItem
  ) => void
  clearCompletedTransfers: (sessionId: string) => void
  cacheDirectory: (sessionId: string, path: string, payload: SftpListResult) => void
  getCachedDirectory: (sessionId: string, path: string) => SftpListResult | undefined
  clearDirectoryCache: (sessionId: string, path?: string) => void
  reset: () => void
}

const generateId = () => `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`

const normalizePath = (value: string) => {
  const trimmed = value.trim()
  if (!trimmed) {
    return ''
  }
  // Handle absolute paths
  if (trimmed.startsWith('/')) {
    const cleaned = trimmed.replace(/\/+/g, '/').replace(/\/+$/, '')
    return cleaned || '/'
  }
  // Handle relative paths or legacy dot notation
  if (trimmed === '.') {
    return ''
  }
  const withoutDouble = trimmed.replace(/\/+/g, '/')
  const cleaned = withoutDouble.replace(/^\/+/, '').replace(/\/+$/, '')
  return cleaned
}

const fileNameFromPath = (path: string) => {
  const cleaned = path.replace(/\/+$/, '')
  const segments = cleaned.split('/')
  return segments[segments.length - 1] || cleaned
}

const MAX_DIRECTORY_CACHE_ENTRIES = 100

export const useSshWorkspaceStore = create<WorkspaceStore>()((set, get) => ({
  sessions: {},
  ensureSession: (sessionId: string) => {
    const existing = get().sessions[sessionId]
    if (existing) {
      return existing
    }
    const browserId = `${sessionId}:browser`
    const session: WorkspaceSessionState = {
      sessionId,
      browserPath: '',
      homeDirectory: undefined,
      showHidden: true,
      tabs: [
        {
          id: browserId,
          type: 'browser',
          title: BASE_BROWSER_TITLE,
        },
      ],
      activeTabId: browserId,
      transfers: {},
      transferOrder: [],
      directoryCache: {},
      directoryCacheOrder: [],
    }
    set((state) => ({
      sessions: {
        ...state.sessions,
        [sessionId]: session,
      },
    }))
    return session
  },
  setBrowserPath: (sessionId, path) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            browserPath: normalizePath(path),
          },
        },
      }
    })
  },
  setHomeDirectory: (sessionId, path) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session || session.homeDirectory) {
        // Only set once
        return state
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            homeDirectory: path,
          },
        },
      }
    })
  },
  setShowHidden: (sessionId, show) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            showHidden: show,
          },
        },
      }
    })
  },
  setActiveTab: (sessionId, tabId) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const tabExists = session.tabs.some((tab) => tab.id === tabId)
      if (!tabExists) {
        return state
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            activeTabId: tabId,
          },
        },
      }
    })
  },
  openEditor: (sessionId, path, title) => {
    get().ensureSession(sessionId)
    set((state) => {
      const current = state.sessions[sessionId]
      if (!current) {
        return state
      }
      const normalizedPath = normalizePath(path)
      const existingTab = current.tabs.find(
        (tab) => tab.type === 'editor' && tab.path === normalizedPath
      )
      if (existingTab) {
        return {
          sessions: {
            ...state.sessions,
            [sessionId]: {
              ...current,
              activeTabId: existingTab.id,
            },
          },
        }
      }
      const editorId = `${sessionId}:editor:${generateId()}`
      const nextTabs: WorkspaceTab[] = [
        ...current.tabs,
        {
          id: editorId,
          type: 'editor',
          title: title || fileNameFromPath(normalizedPath),
          path: normalizedPath,
          dirty: false,
        },
      ]
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...current,
            tabs: nextTabs,
            activeTabId: editorId,
          },
        },
      }
    })
  },
  closeTab: (sessionId, tabId) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      if (session.tabs.length <= 1) {
        return state
      }
      const nextTabs = session.tabs.filter((tab) => tab.id !== tabId)
      const nextActive =
        session.activeTabId === tabId
          ? (nextTabs.find((tab) => tab.type === 'browser')?.id ?? nextTabs[0].id)
          : session.activeTabId
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            tabs: nextTabs,
            activeTabId: nextActive ?? session.activeTabId,
          },
        },
      }
    })
  },
  setTabDirty: (sessionId, tabId, dirty) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const nextTabs = session.tabs.map((tab) => (tab.id === tabId ? { ...tab, dirty } : tab))
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            tabs: nextTabs,
          },
        },
      }
    })
  },
  upsertTransfer: (sessionId, transfer) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const existing = session.transfers[transfer.id]
      const transfers = {
        ...session.transfers,
        [transfer.id]: existing ? { ...existing, ...transfer } : { ...transfer },
      }
      const order = session.transferOrder.includes(transfer.id)
        ? session.transferOrder
        : [...session.transferOrder, transfer.id]
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            transfers,
            transferOrder: order,
          },
        },
      }
    })
  },
  updateTransfer: (sessionId, transferId, updater) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const currentTransfer = session.transfers[transferId]
      if (!currentTransfer) {
        return state
      }
      const nextTransfer = updater(currentTransfer)
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            transfers: {
              ...session.transfers,
              [transferId]: nextTransfer,
            },
          },
        },
      }
    })
  },
  clearCompletedTransfers: (sessionId) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const nextTransfers: Record<string, TransferItem> = {}
      const nextOrder: string[] = []
      for (const id of session.transferOrder) {
        const transfer = session.transfers[id]
        if (!transfer) {
          continue
        }
        if (transfer.status === 'completed' || transfer.status === 'failed') {
          continue
        }
        nextTransfers[id] = transfer
        nextOrder.push(id)
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            transfers: nextTransfers,
            transferOrder: nextOrder,
          },
        },
      }
    })
  },
  cacheDirectory: (sessionId, path, payload) => {
    const normalized = normalizePath(path)
    const timestamp = Date.now()
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      const nextCache = {
        ...session.directoryCache,
        [normalized]: { data: payload, fetchedAt: timestamp },
      }
      const nextOrder = session.directoryCacheOrder.filter((key) => key !== normalized)
      nextOrder.push(normalized)
      while (nextOrder.length > MAX_DIRECTORY_CACHE_ENTRIES) {
        const evicted = nextOrder.shift()
        if (evicted && evicted !== normalized) {
          delete nextCache[evicted]
        }
      }
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            directoryCache: nextCache,
            directoryCacheOrder: nextOrder,
          },
        },
      }
    })
  },
  getCachedDirectory: (sessionId, path) => {
    const normalized = normalizePath(path)
    const session = get().sessions[sessionId]
    const cached = session?.directoryCache?.[normalized]
    if (!session || !cached) {
      return undefined
    }
    if (session.directoryCacheOrder[session.directoryCacheOrder.length - 1] !== normalized) {
      set((state) => {
        const current = state.sessions[sessionId]
        if (!current || !current.directoryCache[normalized]) {
          return state
        }
        const updatedOrder = current.directoryCacheOrder.filter((key) => key !== normalized)
        updatedOrder.push(normalized)
        return {
          sessions: {
            ...state.sessions,
            [sessionId]: {
              ...current,
              directoryCacheOrder: updatedOrder,
            },
          },
        }
      })
    }
    return cached.data
  },
  clearDirectoryCache: (sessionId, path) => {
    set((state) => {
      const session = state.sessions[sessionId]
      if (!session) {
        return state
      }
      if (!path) {
        return {
          sessions: {
            ...state.sessions,
            [sessionId]: {
              ...session,
              directoryCache: {},
              directoryCacheOrder: [],
            },
          },
        }
      }
      const normalized = normalizePath(path)
      if (!session.directoryCache[normalized]) {
        return state
      }
      const nextCache = { ...session.directoryCache }
      delete nextCache[normalized]
      const nextOrder = session.directoryCacheOrder.filter((key) => key !== normalized)
      return {
        sessions: {
          ...state.sessions,
          [sessionId]: {
            ...session,
            directoryCache: nextCache,
            directoryCacheOrder: nextOrder,
          },
        },
      }
    })
  },
  reset: () => {
    set(() => ({
      sessions: {},
    }))
  },
}))

export const resetSshWorkspaceStore = () => {
  useSshWorkspaceStore.getState().reset()
}

export const selectWorkspaceSession = (sessionId: string) => (state: WorkspaceStore) =>
  state.sessions[sessionId]

export const selectWorkspaceTransfers = (sessionId: string) => {
  const store = useSshWorkspaceStore.getState()
  const session = store.sessions[sessionId]
  if (!session) {
    return []
  }
  return session.transferOrder
    .map((id) => session.transfers[id])
    .filter((transfer): transfer is TransferItem => Boolean(transfer))
}
