import { Fragment, useMemo, useState } from 'react'
import {
  Terminal,
  FolderOpen,
  Users,
  UserPlus,
  Key,
  MoreHorizontal,
  Maximize2,
  Minimize2,
  Command as CommandIcon,
  ExternalLink,
  Wand2,
  LayoutGrid,
  Info,
} from 'lucide-react'

import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
import type { ActiveConnectionSession, ActiveSessionParticipant } from '@/types/connections'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/DropdownMenu'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/Modal'

interface SnippetGroup {
  label: string
  snippets: Array<{ id: string; name: string; description?: string }>
}

interface SshWorkspaceUnifiedHeaderProps {
  // Session info
  session: ActiveConnectionSession
  participants?: Record<string, ActiveSessionParticipant>
  currentUserId?: string
  canShare?: boolean
  onOpenShare?: () => void

  // Tabs
  tabs: WorkspaceTab[]
  activeTabId: string
  onSelectTab: (tabId: string) => void
  canUseSftp?: boolean
  onOpenFileManager?: () => void

  // Toolbar actions
  layoutColumns: number
  layoutOptions: number[]
  onLayoutChange: (columns: number) => void
  snippetGroups: SnippetGroup[]
  disabledSnippets: boolean
  onExecuteSnippet: (snippetId: string) => void
  isFullscreen: boolean
  onToggleFullscreen: () => void
  onOpenCommandPalette: () => void
  onOpenNewWindow: () => void
  showSnippetsButton: boolean
}

export function SshWorkspaceUnifiedHeader({
  session,
  participants,
  currentUserId,
  canShare,
  onOpenShare,
  tabs,
  activeTabId,
  onSelectTab,
  canUseSftp,
  onOpenFileManager,
  layoutColumns,
  layoutOptions,
  onLayoutChange,
  snippetGroups,
  disabledSnippets,
  onExecuteSnippet,
  isFullscreen,
  onToggleFullscreen,
  onOpenCommandPalette,
  onOpenNewWindow,
  showSnippetsButton,
}: SshWorkspaceUnifiedHeaderProps) {
  const [sessionInfoOpen, setSessionInfoOpen] = useState(false)

  const participantList = useMemo(() => {
    if (!participants) {
      return []
    }
    const ownerId = session.owner_user_id ?? ''
    const writeHolderId = session.write_holder ?? ''
    return Object.values(participants)
      .map((participant) => ({
        ...participant,
        is_owner: participant.is_owner ?? participant.user_id === ownerId,
        is_write_holder: participant.is_write_holder ?? participant.user_id === writeHolderId,
        joinedAt: participant.joined_at ? new Date(participant.joined_at) : undefined,
      }))
      .sort((a, b) => {
        if (!a.joinedAt || !b.joinedAt) {
          return 0
        }
        return a.joinedAt.getTime() - b.joinedAt.getTime()
      })
  }, [participants, session.owner_user_id, session.write_holder])

  const isOwner =
    currentUserId && session.owner_user_id ? session.owner_user_id === currentUserId : false

  const shareButtonVisible = Boolean(onOpenShare) && (canShare || isOwner)

  const terminalTab = tabs.find((tab) => tab.type === 'terminal')
  const sftpTab = tabs.find((tab) => tab.type === 'sftp')
  const hasSftpTab = Boolean(sftpTab)

  return (
    <div className="flex items-center justify-between gap-3 border-b border-border/50 bg-muted/20 px-3 py-2">
      {/* Left: Tabs */}
      <div className="flex items-center gap-1">
        {/* Terminal tab - always shown */}
        {terminalTab && (
          <button
            onClick={() => onSelectTab(terminalTab.id)}
            className={cn(
              'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              terminalTab.id === activeTabId
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:bg-background/60 hover:text-foreground'
            )}
          >
            <Terminal className="h-3.5 w-3.5" />
            <span>{terminalTab.title}</span>
          </button>
        )}

        {/* SFTP tab - shown if tab exists or if SFTP is available */}
        {(hasSftpTab || canUseSftp) && (
          <button
            onClick={() => {
              if (hasSftpTab && sftpTab) {
                onSelectTab(sftpTab.id)
              } else if (onOpenFileManager) {
                onOpenFileManager()
              }
            }}
            className={cn(
              'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              hasSftpTab && sftpTab?.id === activeTabId
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:bg-background/60 hover:text-foreground'
            )}
          >
            <FolderOpen className="h-3.5 w-3.5" />
            <span>{hasSftpTab && sftpTab ? sftpTab.title : 'Files'}</span>
          </button>
        )}
      </div>

      {/* Right: Compact toolbar */}
      <div className="flex items-center gap-1">
        {/* Session Info Dialog */}
        <Dialog open={sessionInfoOpen} onOpenChange={setSessionInfoOpen}>
          <DialogTrigger asChild>
            <Button variant="ghost" size="sm" className="gap-1.5">
              <Info className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">{session.connection_name ?? 'Session'}</span>
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Session Information</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <div className="text-sm font-medium">Connection</div>
                <div className="text-sm text-muted-foreground">
                  {session.connection_name ?? 'SSH Session'}
                </div>
              </div>
              <div>
                <div className="text-sm font-medium">Details</div>
                <div className="text-sm text-muted-foreground">
                  {session.user_name ?? session.user_id}
                  {session.host ? ` @ ${session.host}` : ''}
                  {session.port && session.port !== 22 ? `:${session.port}` : ''}
                </div>
              </div>
              {participantList.length > 0 && (
                <div>
                  <div className="mb-2 text-sm font-medium">Participants</div>
                  <div className="flex flex-wrap gap-2">
                    {participantList.map((participant) => (
                      <Badge
                        key={participant.user_id}
                        variant={participant.is_write_holder ? 'default' : 'secondary'}
                        className={cn('gap-1', participant.is_write_holder && 'pl-1.5')}
                      >
                        {participant.is_write_holder && <Key className="h-3 w-3" aria-hidden />}
                        {participant.user_name ?? participant.user_id}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
              {shareButtonVisible && (
                <Button onClick={onOpenShare} className="w-full gap-2">
                  <UserPlus className="h-4 w-4" />
                  Share Session
                </Button>
              )}
            </div>
          </DialogContent>
        </Dialog>

        {/* Quick participant count (visible inline) */}
        {participantList.length > 0 && (
          <Button
            variant="ghost"
            size="sm"
            className="hidden gap-1.5 md:flex"
            onClick={() => setSessionInfoOpen(true)}
          >
            <Users className="h-3.5 w-3.5" />
            <span className="text-xs">{participantList.length}</span>
          </Button>
        )}

        {/* More menu */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" aria-label="More options">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            {/* Layout */}
            <DropdownMenuLabel>Layout</DropdownMenuLabel>
            {layoutOptions.map((option) => {
              const active = option === layoutColumns
              return (
                <DropdownMenuItem
                  key={option}
                  onSelect={() => onLayoutChange(option)}
                  className="flex items-center justify-between"
                >
                  <div className="flex items-center gap-2">
                    <LayoutGrid className="h-3.5 w-3.5" />
                    <span>
                      {option} column{option > 1 ? 's' : ''}
                    </span>
                  </div>
                  {active && <span className="text-xs text-muted-foreground">✓</span>}
                </DropdownMenuItem>
              )
            })}

            {/* Snippets */}
            {showSnippetsButton && snippetGroups.length > 0 && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuLabel>Snippets</DropdownMenuLabel>
                {snippetGroups.map((group) => (
                  <Fragment key={group.label}>
                    {group.snippets.map((snippet) => (
                      <DropdownMenuItem
                        key={snippet.id}
                        onSelect={() => onExecuteSnippet(snippet.id)}
                        disabled={disabledSnippets}
                        className="flex flex-col items-start gap-0.5"
                      >
                        <div className="flex items-center gap-2">
                          <Wand2 className="h-3.5 w-3.5" />
                          <span className="text-sm font-medium">{snippet.name}</span>
                        </div>
                        {snippet.description && (
                          <span className="ml-5 text-xs text-muted-foreground">
                            {snippet.description}
                          </span>
                        )}
                      </DropdownMenuItem>
                    ))}
                  </Fragment>
                ))}
              </>
            )}

            <DropdownMenuSeparator />

            {/* View options */}
            <DropdownMenuItem onSelect={onToggleFullscreen}>
              <div className="flex items-center gap-2">
                {isFullscreen ? (
                  <>
                    <Minimize2 className="h-3.5 w-3.5" />
                    <span>Exit Fullscreen</span>
                  </>
                ) : (
                  <>
                    <Maximize2 className="h-3.5 w-3.5" />
                    <span>Fullscreen</span>
                  </>
                )}
              </div>
            </DropdownMenuItem>

            <DropdownMenuItem onSelect={onOpenNewWindow}>
              <div className="flex items-center gap-2">
                <ExternalLink className="h-3.5 w-3.5" />
                <span>Open in New Window</span>
              </div>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Command palette - always visible */}
        <Button
          variant="ghost"
          size="sm"
          onClick={onOpenCommandPalette}
          aria-label="Open command palette"
        >
          <CommandIcon className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}

export type { SnippetGroup }
export default SshWorkspaceUnifiedHeader
