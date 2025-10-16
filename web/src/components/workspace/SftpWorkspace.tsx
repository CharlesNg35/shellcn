import { Suspense, lazy, useCallback, useEffect, useMemo } from 'react'
import { Card } from '@/components/ui/Card'
import { useSshWorkspaceStore } from '@/store/ssh-workspace-store'
import type { ActiveSessionParticipant } from '@/types/connections'
import type { TransferItem } from '@/store/ssh-workspace-store'
import type { SftpEntry } from '@/types/sftp'
import { SftpWorkspaceTabs } from './SftpWorkspaceTabs'
import FileManager from '@/components/file-manager/FileManager'
import { TransferSidebar } from '@/components/file-manager/TransferSidebar'

const LazySftpFileEditor = lazy(() =>
  import('./SftpFileEditor').then((module) => ({ default: module.SftpFileEditor }))
)

function EditorFallback() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      Loading editor…
    </div>
  )
}

interface SftpWorkspaceProps {
  sessionId: string
  canWrite: boolean
  currentUserId?: string
  currentUserName?: string
  participants?: Record<string, ActiveSessionParticipant>
}

export function SftpWorkspace({
  sessionId,
  canWrite,
  currentUserId,
  currentUserName,
  participants,
}: SftpWorkspaceProps) {
  const ensureSession = useSshWorkspaceStore((state) => state.ensureSession)
  const setActiveTab = useSshWorkspaceStore((state) => state.setActiveTab)
  const openEditor = useSshWorkspaceStore((state) => state.openEditor)
  const closeTab = useSshWorkspaceStore((state) => state.closeTab)
  const clearSessionTransfers = useSshWorkspaceStore((state) => state.clearCompletedTransfers)

  useEffect(() => {
    ensureSession(sessionId)
  }, [ensureSession, sessionId])

  const sessionState = useSshWorkspaceStore((state) => state.sessions[sessionId])

  const transferOrder = useSshWorkspaceStore(
    (state) => state.sessions[sessionId]?.transferOrder ?? []
  )
  const transfersMap = useSshWorkspaceStore(
    (state) => state.sessions[sessionId]?.transfers ?? ({} as Record<string, TransferItem>)
  )

  const transfers = useMemo(
    () =>
      transferOrder
        .map((id) => transfersMap[id])
        .filter((transfer): transfer is TransferItem => Boolean(transfer)),
    [transferOrder, transfersMap]
  )

  const activeTab = sessionState?.tabs.find((tab) => tab.id === sessionState.activeTabId)

  const handleOpenFile = useCallback(
    (entry: SftpEntry) => {
      openEditor(sessionId, entry.path, entry.name)
    },
    [openEditor, sessionId]
  )

  const handleSelectTab = useCallback(
    (tabId: string) => {
      setActiveTab(sessionId, tabId)
    },
    [sessionId, setActiveTab]
  )

  const handleCloseTab = useCallback(
    (tabId: string) => {
      closeTab(sessionId, tabId)
    },
    [closeTab, sessionId]
  )

  return (
    <div className="flex h-full flex-col gap-4">
      <Card className="border border-border bg-card p-2 shadow-sm">
        <SftpWorkspaceTabs
          tabs={sessionState?.tabs ?? []}
          activeTabId={sessionState?.activeTabId ?? ''}
          onSelect={handleSelectTab}
          onClose={handleCloseTab}
        />
      </Card>

      <div className="flex flex-1 gap-4 overflow-hidden">
        <div className="flex-1 overflow-hidden">
          {activeTab?.type === 'editor' && activeTab.path ? (
            <Suspense fallback={<EditorFallback />}>
              <LazySftpFileEditor
                sessionId={sessionId}
                tabId={activeTab.id}
                path={activeTab.path}
                canWrite={canWrite}
              />
            </Suspense>
          ) : (
            <FileManager
              sessionId={sessionId}
              canWrite={canWrite}
              currentUserId={currentUserId}
              currentUserName={currentUserName}
              participants={participants}
              onOpenFile={handleOpenFile}
              showTransfers={false}
            />
          )}
        </div>
        <TransferSidebar transfers={transfers} onClear={() => clearSessionTransfers(sessionId)} />
      </div>
    </div>
  )
}
