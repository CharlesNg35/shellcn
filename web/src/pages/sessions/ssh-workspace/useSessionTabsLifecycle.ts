import { useEffect } from 'react'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SessionWorkspaceState } from '@/store/ssh-session-tabs-store'

interface UseSessionTabsLifecycleParams {
  session?: ActiveConnectionSession
  workspace?: SessionWorkspaceState
  onOpenSession: () => void
  ensureTerminalTab: () => void
  ensureSftpTab?: () => void
}

export function useSessionTabsLifecycle({
  session,
  workspace,
  onOpenSession,
  ensureTerminalTab,
  ensureSftpTab,
}: UseSessionTabsLifecycleParams) {
  useEffect(() => {
    if (!session || workspace) {
      return
    }
    onOpenSession()
  }, [onOpenSession, session, workspace])

  useEffect(() => {
    if (!session || !workspace) {
      return
    }
    ensureTerminalTab()
    ensureSftpTab?.()
  }, [ensureSftpTab, ensureTerminalTab, session, workspace])
}
