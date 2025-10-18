import { Suspense, lazy, type RefObject } from 'react'
import { Tabs, TabsContent } from '@/components/ui/Tabs'
import type { SshTerminalHandle } from '@/components/workspace/SshTerminal'
import SshWorkspaceStatusBar from '@/components/workspace/ssh/SshWorkspaceStatusBar'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SessionRecordingStatus } from '@/types/session-recording'
import type { TerminalSearchControls } from './useTerminalSearch'
import type { WorkspaceTelemetryControls } from './useWorkspaceTelemetry'
import type { SessionTunnelEntry } from '@/store/ssh-session-tunnel-store'

const LazySshTerminal = lazy(() =>
  import('@/components/workspace/SshTerminal').then((module) => ({ default: module.SshTerminal }))
)
const LazySftpWorkspace = lazy(() =>
  import('@/components/workspace/SftpWorkspace').then((module) => ({
    default: module.SftpWorkspace,
  }))
)

function TerminalFallback() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      Preparing terminal…
    </div>
  )
}

function SftpFallback() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      Loading file manager…
    </div>
  )
}

interface SshWorkspaceContentProps {
  sessionId: string
  tabs: WorkspaceTab[]
  activeTabId: string
  onSelectTab: (tabId: string) => void
  terminalRef: React.RefObject<SshTerminalHandle | null>
  search: TerminalSearchControls
  telemetry: WorkspaceTelemetryControls
  canWrite: boolean
  currentUserId?: string
  currentUserName?: string
  participants?: ActiveConnectionSession['participants']
  recordingActive: boolean
  recordingStatus?: SessionRecordingStatus
  recordingLoading?: boolean
  onRecordingDetails?: () => void
  transfers: {
    active: number
    total: number
  }
  tunnel?: SessionTunnelEntry
}

export function SshWorkspaceContent({
  sessionId,
  tabs,
  activeTabId,
  onSelectTab,
  terminalRef,
  search,
  telemetry,
  canWrite,
  currentUserId,
  currentUserName,
  participants,
  recordingActive,
  recordingStatus,
  recordingLoading,
  onRecordingDetails,
  transfers,
  tunnel,
}: SshWorkspaceContentProps) {
  return (
    <Tabs
      value={activeTabId}
      onValueChange={onSelectTab}
      className="flex h-full flex-1 flex-col overflow-hidden"
    >
      <div className="flex flex-1 flex-col overflow-hidden rounded-xl border border-border bg-background/60 shadow-inner">
        <div className="flex-1 overflow-hidden">
          {tabs.map((tab) => (
            <TabsContent key={tab.id} value={tab.id} className="h-full w-full" forceMount>
              {tab.type === 'terminal' ? (
                <Suspense fallback={<TerminalFallback />}>
                  <LazySshTerminal
                    ref={terminalRef}
                    sessionId={sessionId}
                    tunnel={tunnel}
                    onEvent={telemetry.handleTerminalEvent}
                    onFontSizeChange={telemetry.setFontSize}
                    searchOverlay={search.overlay}
                    onSearchResolved={({ matched }) => search.onResolved(matched)}
                    activeTabId={activeTabId}
                  />
                </Suspense>
              ) : (
                <Suspense fallback={<SftpFallback />}>
                  <LazySftpWorkspace
                    sessionId={sessionId}
                    canWrite={canWrite}
                    currentUserId={currentUserId}
                    currentUserName={currentUserName}
                    participants={participants}
                  />
                </Suspense>
              )}
            </TabsContent>
          ))}
        </div>

        <SshWorkspaceStatusBar
          fontSize={telemetry.fontSize}
          onZoomIn={telemetry.zoomIn}
          onZoomOut={telemetry.zoomOut}
          onZoomReset={telemetry.zoomReset}
          onToggleSearch={search.toggle}
          isSearchOpen={search.isOpen}
          searchQuery={search.query}
          onSearchQueryChange={search.onQueryChange}
          searchDirection={search.direction}
          onSearchDirectionChange={search.onDirectionChange}
          onSearchSubmit={search.onSubmit}
          searchMatched={search.matched}
          latencyMs={telemetry.latencyMs}
          lastActivityAt={telemetry.lastActivityAt}
          transfers={transfers}
          recordingActive={recordingActive}
          recordingStatus={recordingStatus}
          recordingLoading={recordingLoading}
          onRecordingDetails={onRecordingDetails}
          searchInputRef={search.inputRef as RefObject<HTMLInputElement>}
        />
      </div>
    </Tabs>
  )
}

export default SshWorkspaceContent
