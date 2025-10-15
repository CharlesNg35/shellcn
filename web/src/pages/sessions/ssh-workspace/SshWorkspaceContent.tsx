import type { RefObject } from 'react'
import { Tabs, TabsContent } from '@/components/ui/Tabs'
import { SshTerminal, type SshTerminalHandle } from '@/components/workspace/SshTerminal'
import { SftpWorkspace } from '@/components/workspace/SftpWorkspace'
import SshWorkspaceStatusBar from '@/components/workspace/ssh/SshWorkspaceStatusBar'
import SshWorkspaceTabsBar from '@/components/workspace/ssh/SshWorkspaceTabsBar'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SessionRecordingStatus } from '@/types/session-recording'
import type { TerminalSearchControls } from './useTerminalSearch'
import type { WorkspaceTelemetryControls } from './useWorkspaceTelemetry'

interface SshWorkspaceContentProps {
  sessionId: string
  tabs: WorkspaceTab[]
  activeTabId: string
  layoutColumns: number
  onSelectTab: (tabId: string) => void
  onCloseTab: (tabId: string) => void
  onReorderTabs: (orderedIds: string[]) => void
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
}

export function SshWorkspaceContent({
  sessionId,
  tabs,
  activeTabId,
  layoutColumns,
  onSelectTab,
  onCloseTab,
  onReorderTabs,
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
}: SshWorkspaceContentProps) {
  return (
    <Tabs
      value={activeTabId}
      onValueChange={onSelectTab}
      className="flex h-full flex-1 flex-col overflow-hidden"
    >
      <div className="flex flex-1 flex-col overflow-hidden rounded-xl border border-border bg-background/60 shadow-inner">
        <div className="border-b border-border/60 px-3 py-2">
          <SshWorkspaceTabsBar
            tabs={tabs}
            activeTabId={activeTabId}
            onTabSelect={onSelectTab}
            onTabClose={onCloseTab}
            onTabsReordered={onReorderTabs}
          />
        </div>

        <div className="flex-1 overflow-hidden">
          {tabs.map((tab) => (
            <TabsContent key={tab.id} value={tab.id} className="h-full w-full" forceMount>
              <div
                className="grid h-full gap-4 px-4 py-4"
                style={{ gridTemplateColumns: `repeat(${layoutColumns}, minmax(0, 1fr))` }}
                data-columns={layoutColumns}
                data-testid={tab.type === 'terminal' ? 'terminal-grid' : undefined}
              >
                {tab.type === 'terminal' ? (
                  <div className="col-span-full h-full">
                    <SshTerminal
                      ref={terminalRef}
                      sessionId={sessionId}
                      onEvent={telemetry.handleTerminalEvent}
                      onFontSizeChange={telemetry.setFontSize}
                      searchOverlay={search.overlay}
                      onSearchResolved={({ matched }) => search.onResolved(matched)}
                    />
                  </div>
                ) : (
                  <div className="col-span-full h-full">
                    <SftpWorkspace
                      sessionId={sessionId}
                      canWrite={canWrite}
                      currentUserId={currentUserId}
                      currentUserName={currentUserName}
                      participants={participants}
                    />
                  </div>
                )}
              </div>
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
