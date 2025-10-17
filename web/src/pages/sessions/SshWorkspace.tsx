import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Loader2 } from 'lucide-react'

import { EmptyState } from '@/components/ui/EmptyState'
import SshWorkspaceToolbar from '@/components/workspace/ssh/SshWorkspaceToolbar'
import SshCommandPalette from '@/components/workspace/ssh/SshCommandPalette'
import { PERMISSIONS } from '@/constants/permissions'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { usePermissions } from '@/hooks/usePermissions'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { useSshWorkspaceTabsStore } from '@/store/ssh-session-tabs-store'
import { useSshWorkspaceStore } from '@/store/ssh-workspace-store'
import type { SshTerminalHandle } from '@/components/workspace/SshTerminal'
import { cn } from '@/lib/utils/cn'
import { sessionSupportsSftp } from '@/lib/utils/sessionCapabilities'

import { useActiveSession } from '@/hooks/useActiveSession'
import { useSessionTabsLifecycle } from './ssh-workspace/useSessionTabsLifecycle'
import { useWorkspaceSnippets } from './ssh-workspace/useWorkspaceSnippets'
import { useCommandPaletteState } from './ssh-workspace/useCommandPaletteState'
import { useTerminalSearch } from './ssh-workspace/useTerminalSearch'
import { useWorkspaceTelemetry } from './ssh-workspace/useWorkspaceTelemetry'
import { useSessionRecording } from '@/hooks/useSessionRecording'
import SshWorkspaceHeader from './ssh-workspace/SshWorkspaceHeader'
import { SessionShareDialog } from './ssh-workspace/SessionShareDialog'
import SshWorkspaceContent from './ssh-workspace/SshWorkspaceContent'
import { SessionRecordingDialog } from './ssh-workspace/SessionRecordingDialog'

const LAYOUT_OPTIONS = [1, 2, 3, 4, 5]

function resolveDisplayName(user?: {
  first_name?: string | null
  last_name?: string | null
  username?: string | null
  email?: string | null
  id: string
}) {
  if (!user) {
    return undefined
  }
  const parts = [user.first_name, user.last_name].filter(Boolean).join(' ').trim()
  if (parts) {
    return parts
  }
  return user.username ?? user.email ?? user.id
}

export function SshWorkspace() {
  const { sessionId = '' } = useParams<{ sessionId: string }>()
  const navigate = useNavigate()
  const { setOverride, clearOverride } = useBreadcrumb()
  const { hasPermission } = usePermissions()

  const {
    session,
    sessions: activeSessions,
    isLoading,
    isError,
  } = useActiveSession(sessionId, {
    protocolId: 'ssh',
  })
  const {
    status: recordingStatus,
    isLoading: recordingStatusLoading,
    refetch: refetchRecordingStatus,
  } = useSessionRecording(session?.id ?? null, {
    enabled: Boolean(session?.id),
  })

  const openSession = useSshWorkspaceTabsStore((state) => state.openSession)
  const ensureTab = useSshWorkspaceTabsStore((state) => state.ensureTab)
  const closeTab = useSshWorkspaceTabsStore((state) => state.closeTab)
  const reorderTabs = useSshWorkspaceTabsStore((state) => state.reorderTabs)
  const setActiveTab = useSshWorkspaceTabsStore((state) => state.setActiveTab)
  const setLayoutColumns = useSshWorkspaceTabsStore((state) => state.setLayoutColumns)
  const setFullscreen = useSshWorkspaceTabsStore((state) => state.setFullscreen)
  const orderedSessionIds = useSshWorkspaceTabsStore((state) => state.orderedSessionIds)
  const workspace = useSshWorkspaceTabsStore((state) => state.sessions[sessionId])
  const tabs = workspace?.tabs ?? []
  const activeTabId = workspace?.activeTabId ?? ''
  const layoutColumns = workspace?.layoutColumns ?? 1
  const isFullscreen = workspace?.isFullscreen ?? false

  const terminalRef = useRef<SshTerminalHandle | null>(null)

  const currentUserQuery = useCurrentUser()
  const currentUser = currentUserQuery.data ?? undefined
  const currentUserId = currentUser?.id
  const currentUserDisplayName = resolveDisplayName(currentUser)

  const canUseSnippets = hasPermission(PERMISSIONS.PROTOCOL.SSH.MANAGE_SNIPPETS)
  const sftpSupported = sessionSupportsSftp(session)
  const canUseSftp = hasPermission(PERMISSIONS.PROTOCOL.SSH.SFTP) && sftpSupported
  const canShareSession = hasPermission(PERMISSIONS.PROTOCOL.SSH.SHARE)
  const canGrantWrite = hasPermission(PERMISSIONS.PROTOCOL.SSH.GRANT_WRITE)

  const [shareDialogOpen, setShareDialogOpen] = useState(false)
  const [recordingDialogOpen, setRecordingDialogOpen] = useState(false)

  const logEvent = useCallback((action: string, details?: Record<string, unknown>) => {
    if (import.meta.env.DEV) {
      console.info('[ssh-workspace]', action, details ?? {})
    }
  }, [])

  useSessionTabsLifecycle({
    session,
    workspace,
    onOpenSession: () => {
      if (!session) {
        return
      }
      openSession({
        sessionId: session.id,
        connectionId: session.connection_id,
        connectionName: session.connection_name,
      })
    },
    ensureTerminalTab: () => {
      if (!session) {
        return
      }
      ensureTab(session.id, 'terminal', { title: 'Terminal', closable: false })
    },
    ensureSftpTab: canUseSftp
      ? () => {
          if (!session) {
            return
          }
          ensureTab(session.id, 'sftp', { title: 'Files', closable: true })
        }
      : undefined,
  })

  useEffect(() => {
    if (!session || canUseSftp) {
      return
    }
    const tabs = workspace?.tabs ?? []
    tabs
      .filter((tab) => tab.type === 'sftp')
      .forEach((tab) => {
        closeTab(session.id, tab.id)
      })
  }, [session, canUseSftp, workspace?.tabs, closeTab])

  useEffect(() => {
    const label = session?.connection_name ?? session?.connection_id
    if (sessionId && label) {
      const path = `/active-sessions/${sessionId}`
      setOverride(path, `${label} · Workspace`)
      return () => {
        clearOverride(path)
      }
    }
    return undefined
  }, [clearOverride, session?.connection_id, session?.connection_name, sessionId, setOverride])

  useEffect(() => {
    setRecordingDialogOpen(false)
  }, [session?.id])

  useEffect(() => {
    if (!isFullscreen) {
      document.body.classList.remove('ssh-workspace-fullscreen')
      document.body.classList.remove('overflow-hidden')
      return
    }
    document.body.classList.add('ssh-workspace-fullscreen')
    document.body.classList.add('overflow-hidden')
    return () => {
      document.body.classList.remove('ssh-workspace-fullscreen')
      document.body.classList.remove('overflow-hidden')
    }
  }, [isFullscreen])

  const sessionTransfersState = useSshWorkspaceStore((state) => state.sessions[sessionId])
  const transfersSummary = useMemo(() => {
    if (!sessionTransfersState) {
      return { active: 0, total: 0 }
    }
    const transfers = sessionTransfersState.transferOrder
      .map((id) => sessionTransfersState.transfers[id])
      .filter((transfer): transfer is NonNullable<typeof transfer> => Boolean(transfer))
    const active = transfers.filter(
      (transfer) => transfer.status === 'pending' || transfer.status === 'uploading'
    ).length
    return { active, total: transfers.length }
  }, [sessionTransfersState])

  const telemetry = useWorkspaceTelemetry({ terminalRef, logEvent })

  const performSearch = useCallback(
    (query: string, direction: 'next' | 'previous') =>
      terminalRef.current?.search(query, direction) ?? false,
    []
  )

  const searchControls = useTerminalSearch({
    performSearch,
    logEvent,
    sessionId,
  })

  const commandPalette = useCommandPaletteState({
    tabs,
    activeTabId,
    sessionId,
    orderedSessionIds,
    activeSessions,
    setActiveTab,
    navigate,
  })

  const {
    groups: snippetGroups,
    isLoading: snippetsLoading,
    snippetsAvailable,
    executeSnippet,
    isExecuting: snippetExecuting,
  } = useWorkspaceSnippets({
    session,
    enabled: canUseSnippets,
    logEvent,
  })

  const recordingActive = useMemo(() => {
    if (recordingStatus) {
      return recordingStatus.active
    }
    const metadata = session?.metadata ?? {}
    if (typeof metadata !== 'object' || metadata === null) {
      return false
    }
    if ('recording_active' in metadata) {
      return Boolean((metadata as Record<string, unknown>).recording_active)
    }
    if ('recording' in metadata && typeof metadata.recording === 'object') {
      return Boolean((metadata.recording as Record<string, unknown>).active)
    }
    return false
  }, [recordingStatus, session?.metadata])

  const canWrite = useMemo(() => {
    if (!session || !currentUserId) {
      return false
    }
    if (session.write_holder && session.write_holder === currentUserId) {
      return true
    }
    if (session.owner_user_id && session.owner_user_id === currentUserId) {
      return true
    }
    if (session.user_id === currentUserId) {
      return true
    }
    const participant = session.participants?.[currentUserId]
    return participant?.access_mode?.toLowerCase() === 'write'
  }, [currentUserId, session])

  const handleSelectTab = useCallback(
    (value: string) => {
      if (!sessionId || !value) {
        return
      }
      setActiveTab(sessionId, value)
    },
    [sessionId, setActiveTab]
  )

  const handleTabClose = useCallback(
    (tabId: string) => {
      if (!sessionId) {
        return
      }
      closeTab(sessionId, tabId)
    },
    [closeTab, sessionId]
  )

  const handleTabReorder = useCallback(
    (orderedTabIds: string[]) => {
      if (!sessionId) {
        return
      }
      reorderTabs(sessionId, orderedTabIds)
      logEvent('tabs.reordered', { sessionId, order: orderedTabIds })
    },
    [logEvent, reorderTabs, sessionId]
  )

  const handleRecordingDetails = useCallback(() => {
    if (!session) {
      return
    }
    setRecordingDialogOpen(true)
  }, [session])

  const handleLayoutSelect = useCallback(
    (columns: number) => {
      if (!sessionId) {
        return
      }
      setLayoutColumns(sessionId, columns)
      logEvent('layout.columns', { sessionId, columns })
    },
    [logEvent, sessionId, setLayoutColumns]
  )

  const handleExecuteSnippet = useCallback(
    (snippetId: string) => {
      executeSnippet(snippetId)
    },
    [executeSnippet]
  )

  const handleOpenFileManager = useCallback(() => {
    if (!session) {
      return
    }
    const tab = ensureTab(session.id, 'sftp', { title: 'Files', closable: true })
    setActiveTab(session.id, tab.id)
    logEvent('file_manager.open', { sessionId })
  }, [ensureTab, logEvent, session, sessionId, setActiveTab])

  const handleToggleFullscreen = useCallback(() => {
    if (!sessionId) {
      return
    }
    setFullscreen(sessionId, !isFullscreen)
    logEvent('fullscreen.toggle', { sessionId, enabled: !isFullscreen })
  }, [isFullscreen, logEvent, sessionId, setFullscreen])

  const handleOpenNewWindow = useCallback(() => {
    if (!sessionId) {
      return
    }
    const url = new URL(window.location.href)
    url.pathname = `/active-sessions/${sessionId}`
    window.open(url.toString(), '_blank', 'noopener,noreferrer')
    logEvent('window.open', { sessionId })
  }, [logEvent, sessionId])

  if (!sessionId) {
    return (
      <EmptyState
        title="Session not specified"
        description="Provide an active session identifier to open the SSH workspace."
        className="h-full"
      />
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
        Loading workspace…
      </div>
    )
  }

  if (isError || !session) {
    return (
      <EmptyState
        title="Session unavailable"
        description="The requested session could not be found or is no longer active."
        className="h-full"
      />
    )
  }

  const snippetButtonDisabled = snippetsLoading || snippetExecuting || !snippetsAvailable

  return (
    <div
      className={cn(
        'flex h-full flex-col gap-6',
        isFullscreen && 'fixed inset-0 z-50 bg-background p-4 lg:p-6'
      )}
    >
      <SshWorkspaceHeader
        session={session}
        participants={session.participants}
        currentUserId={currentUserId}
        canShare={canShareSession}
        onOpenShare={() => setShareDialogOpen(true)}
      />

      <SshWorkspaceToolbar
        layoutColumns={layoutColumns}
        layoutOptions={LAYOUT_OPTIONS}
        onLayoutChange={handleLayoutSelect}
        snippetGroups={snippetGroups}
        loadingSnippets={snippetsLoading}
        disabledSnippets={snippetButtonDisabled}
        onExecuteSnippet={handleExecuteSnippet}
        onOpenFileManager={handleOpenFileManager}
        showFileManagerButton={canUseSftp}
        isFullscreen={isFullscreen}
        onToggleFullscreen={handleToggleFullscreen}
        onOpenCommandPalette={commandPalette.open}
        onOpenNewWindow={handleOpenNewWindow}
        snippetsAvailable={snippetsAvailable}
        showSnippetsButton={canUseSnippets}
      />

      <SshWorkspaceContent
        sessionId={sessionId}
        tabs={tabs}
        activeTabId={activeTabId}
        layoutColumns={layoutColumns}
        onSelectTab={handleSelectTab}
        onCloseTab={handleTabClose}
        onReorderTabs={handleTabReorder}
        terminalRef={terminalRef}
        search={searchControls}
        telemetry={telemetry}
        canWrite={canWrite}
        currentUserId={currentUserId}
        currentUserName={currentUserDisplayName}
        participants={session.participants}
        recordingActive={recordingActive}
        recordingStatus={recordingStatus}
        recordingLoading={recordingStatusLoading}
        onRecordingDetails={session ? handleRecordingDetails : undefined}
        transfers={transfersSummary}
      />

      <SshCommandPalette
        open={commandPalette.isOpen}
        onClose={commandPalette.close}
        tabs={commandPalette.paletteTabs}
        sessions={commandPalette.paletteSessions}
      />

      <SessionShareDialog
        sessionId={sessionId}
        open={shareDialogOpen}
        onClose={() => setShareDialogOpen(false)}
        session={session}
        currentUserId={currentUserId}
        canShare={canShareSession}
        canGrantWrite={canGrantWrite}
      />

      <SessionRecordingDialog
        open={recordingDialogOpen}
        onClose={() => setRecordingDialogOpen(false)}
        sessionId={session?.id ?? sessionId}
        status={recordingStatus}
        isLoading={recordingStatusLoading}
        onRefresh={refetchRecordingStatus}
      />
    </div>
  )
}

export default SshWorkspace
