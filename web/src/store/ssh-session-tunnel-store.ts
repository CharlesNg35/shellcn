import { create } from 'zustand'

import type { LaunchSessionTunnel } from '@/lib/api/active-sessions'

export interface SessionTunnelEntry extends LaunchSessionTunnel {
  sessionId: string
  connectionId?: string
}

interface SshSessionTunnelStore {
  tunnels: Record<string, SessionTunnelEntry>
  setTunnel: (
    sessionId: string,
    connectionId: string | undefined,
    tunnel: LaunchSessionTunnel
  ) => void
  clearTunnel: (sessionId: string) => void
}

export const useSshSessionTunnelStore = create<SshSessionTunnelStore>()((set) => ({
  tunnels: {},
  setTunnel: (sessionId, connectionId, tunnel) => {
    const trimmedId = sessionId.trim()
    if (!trimmedId) {
      return
    }
    set((state) => ({
      tunnels: {
        ...state.tunnels,
        [trimmedId]: {
          sessionId: trimmedId,
          connectionId: connectionId?.trim() || undefined,
          ...tunnel,
        },
      },
    }))
  },
  clearTunnel: (sessionId) => {
    const trimmedId = sessionId.trim()
    if (!trimmedId) {
      return
    }
    set((state) => {
      if (!state.tunnels[trimmedId]) {
        return state
      }
      const next = { ...state.tunnels }
      delete next[trimmedId]
      return { tunnels: next }
    })
  },
}))
