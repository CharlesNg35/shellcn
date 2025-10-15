import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import {
  Command as CommandIcon,
  ExternalLink,
  File as FileIcon,
  LayoutGrid,
  Loader2,
  Maximize2,
  Minimize2,
  Search as SearchIcon,
  Wand2,
  X,
} from 'lucide-react'

import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/DropdownMenu'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { useSnippets, useExecuteSnippet } from '@/hooks/useSnippets'
import { SshTerminal, type SshTerminalHandle } from '@/components/workspace/SshTerminal'
import { SftpWorkspace } from '@/components/workspace/SftpWorkspace'
import {
  selectSessionWorkspace,
  useSshWorkspaceTabsStore,
  type WorkspaceTab,
} from '@/store/ssh-session-tabs-store'
import { useSshWorkspaceStore } from '@/store/ssh-workspace-store'
import { PERMISSIONS } from '@/constants/permissions'
import { toast } from '@/lib/utils/toast'
import { cn } from '@/lib/utils/cn'
import type { SnippetRecord } from '@/lib/api/snippets'

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

function resolveTabLabel(tab: WorkspaceTab) {
  const suffix = tab.meta?.badge ? ` · ${tab.meta.badge}` : ''
  return `${tab.title}${suffix}`
}

type SnippetGroup = {
  label: string
  scope: SnippetRecord['scope'] | 'recent'
  snippets: SnippetRecord[]
}

const LAYOUT_OPTIONS = [1, 2, 3, 4, 5]

function groupSnippets(snippets: SnippetRecord[]): SnippetGroup[] {
  const groups: SnippetGroup[] = []
  const byScope = new Map<SnippetRecord['scope'], SnippetRecord[]>()
  snippets.forEach((snippet) => {
    const scopeGroup = byScope.get(snippet.scope) ?? []
    scopeGroup.push(snippet)
    byScope.set(snippet.scope, scopeGroup)
  })

  const scopeOrder: Array<{ scope: SnippetRecord['scope']; label: string }> = [
    { scope: 'global', label: 'Global snippets' },
    { scope: 'connection', label: 'Connection snippets' },
    { scope: 'user', label: 'Personal snippets' },
  ]

  scopeOrder.forEach(({ scope, label }) => {
    const bucket = byScope.get(scope)
    if (bucket?.length) {
      const sorted = [...bucket].sort((a, b) => a.name.localeCompare(b.name))
      groups.push({ scope, label, snippets: sorted })
    }
  })

  return groups
}

export function SshWorkspace() {
  const { sessionId = '' } = useParams<{ sessionId: string }>()
  const navigate = useNavigate()
  const { setOverride, clearOverride } = useBreadcrumb()

  const {
    data: activeSessions,
    isLoading,
    isError,
  } = useActiveConnections({
    protocol_id: 'ssh',
    enabled: Boolean(sessionId),
    refetchInterval: 15_000,
  })

  const session = useMemo(
    () => activeSessions?.find((record) => record.id === sessionId),
    [activeSessions, sessionId]
  )

  const currentUserQuery = useCurrentUser()
  const currentUser = currentUserQuery.data ?? undefined
  const currentUserId = currentUser?.id
  const currentUserDisplayName = resolveDisplayName(currentUser)

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

  const openSession = useSshWorkspaceTabsStore((state) => state.openSession)
  const ensureTab = useSshWorkspaceTabsStore((state) => state.ensureTab)
  const closeTab = useSshWorkspaceTabsStore((state) => state.closeTab)
  const setActiveTab = useSshWorkspaceTabsStore((state) => state.setActiveTab)
  const setLayoutColumns = useSshWorkspaceTabsStore((state) => state.setLayoutColumns)
  const setFullscreen = useSshWorkspaceTabsStore((state) => state.setFullscreen)
  const orderedSessionIds = useSshWorkspaceTabsStore((state) => state.orderedSessionIds)

  const workspace = useSshWorkspaceTabsStore(selectSessionWorkspace(sessionId))
  const layoutColumns = workspace?.layoutColumns ?? 1
  const isFullscreen = workspace?.isFullscreen ?? false
  const tabs = useMemo(() => workspace?.tabs ?? [], [workspace?.tabs])
  const activeTabId = workspace?.activeTabId ?? ''

  const terminalRef = useRef<SshTerminalHandle | null>(null)
  const lastEventTimestampRef = useRef<number | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)

  const [isSearchOpen, setIsSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchDirection, setSearchDirection] = useState<'next' | 'previous'>('next')
  const [searchMatched, setSearchMatched] = useState(true)
  const [latencyMs, setLatencyMs] = useState<number | null>(null)
  const [lastActivityAt, setLastActivityAt] = useState<Date | null>(null)
  const [fontSize, setFontSize] = useState(14)
  const [isCommandPaletteOpen, setCommandPaletteOpen] = useState(false)

  const logEvent = useCallback((action: string, details?: Record<string, unknown>) => {
    if (import.meta.env.DEV) {
      console.info('[ssh-workspace]', action, details ?? {})
    }
  }, [])

  const hasTerminalTab = useMemo(() => tabs.some((tab) => tab.type === 'terminal'), [tabs])
  const hasSftpTab = useMemo(() => tabs.some((tab) => tab.type === 'sftp'), [tabs])

  useEffect(() => {
    if (!session || workspace) {
      return
    }
    openSession({
      sessionId: session.id,
      connectionId: session.connection_id,
      connectionName: session.connection_name,
    })
  }, [openSession, session?.connection_id, session?.connection_name, session?.id, workspace])

  useEffect(() => {
    if (!session || !workspace) {
      return
    }
    if (!hasTerminalTab) {
      ensureTab(session.id, 'terminal', { title: 'Terminal', closable: false })
    }
    if (!hasSftpTab) {
      ensureTab(session.id, 'sftp', { title: 'Files', closable: true })
    }
  }, [ensureTab, hasSftpTab, hasTerminalTab, session?.id, workspace])

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

  useEffect(() => {
    if (isSearchOpen) {
      const handle = window.setTimeout(() => {
        searchInputRef.current?.focus()
      }, 10)
      return () => window.clearTimeout(handle)
    }
    setSearchMatched(true)
  }, [isSearchOpen])

  const transfersSummary = useSshWorkspaceStore((state) => {
    const sessionState = sessionId ? state.sessions[sessionId] : undefined
    if (!sessionState) {
      return { active: 0, total: 0 }
    }
    const transfers = sessionState.transferOrder
      .map((id) => sessionState.transfers[id])
      .filter((transfer): transfer is NonNullable<typeof transfer> => Boolean(transfer))
    const active = transfers.filter(
      (transfer) => transfer.status === 'pending' || transfer.status === 'uploading'
    ).length
    return { active, total: transfers.length }
  })

  const snippetsQuery = useSnippets({
    enabled: Boolean(session),
    scope: 'all',
    connectionId: session?.connection_id,
  })
  const snippets = useMemo(() => snippetsQuery.data ?? [], [snippetsQuery.data])
  const snippetGroups = useMemo(() => groupSnippets(snippets), [snippets])
  const executeSnippetMutation = useExecuteSnippet({
    onSuccess: () => {
      toast.success('Snippet executed')
      logEvent('snippet.execute.success', { sessionId })
    },
    onError: (error) => {
      toast.error('Failed to execute snippet', { description: error.message })
      logEvent('snippet.execute.error', { sessionId, error: error.message })
    },
  })

  const handleTabChange = (value: string) => {
    if (!sessionId || !value) {
      return
    }
    setActiveTab(sessionId, value)
  }

  const handleCloseTab = (tabId: string, event?: React.MouseEvent | React.KeyboardEvent) => {
    event?.stopPropagation()
    closeTab(sessionId, tabId)
  }

  const handleLayoutSelect = (columns: number) => {
    if (!sessionId) {
      return
    }
    setLayoutColumns(sessionId, columns)
    logEvent('layout.columns', { sessionId, columns })
  }

  const handleSnippetSelect = (snippet: SnippetRecord) => {
    if (!sessionId) {
      return
    }
    executeSnippetMutation.mutate({ sessionId, snippetId: snippet.id })
  }

  const handleOpenFileManager = () => {
    if (!session) {
      return
    }
    const tab = ensureTab(session.id, 'sftp', { title: 'Files', closable: true })
    setActiveTab(session.id, tab.id)
    logEvent('file_manager.open', { sessionId })
  }

  const handleToggleFullscreen = () => {
    if (!sessionId) {
      return
    }
    setFullscreen(sessionId, !isFullscreen)
    logEvent('fullscreen.toggle', { sessionId, enabled: !isFullscreen })
  }

  const handleOpenNewWindow = () => {
    if (!sessionId) {
      return
    }
    const url = new URL(window.location.href)
    url.pathname = `/active-sessions/${sessionId}`
    window.open(url.toString(), '_blank', 'noopener,noreferrer')
    logEvent('window.open', { sessionId })
  }

  const handleZoomIn = () => {
    const next = terminalRef.current?.adjustFontSize(1)
    if (next !== undefined) {
      setFontSize(next)
      logEvent('terminal.zoom', { direction: 'in', fontSize: next })
    }
  }

  const handleZoomOut = () => {
    const next = terminalRef.current?.adjustFontSize(-1)
    if (next !== undefined) {
      setFontSize(next)
      logEvent('terminal.zoom', { direction: 'out', fontSize: next })
    }
  }

  const handleZoomReset = () => {
    const next = terminalRef.current?.setFontSize(14)
    if (next !== undefined) {
      setFontSize(next)
      logEvent('terminal.zoom.reset', { fontSize: next })
    }
  }

  const handleSearchSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!searchQuery) {
      return
    }
    const matched = terminalRef.current?.search(searchQuery, searchDirection) ?? false
    setSearchMatched(matched)
    logEvent('terminal.search', {
      queryLength: searchQuery.length,
      direction: searchDirection,
      matched,
    })
  }

  const handleTerminalEvent = useCallback(() => {
    const now = performance.now()
    if (lastEventTimestampRef.current != null) {
      setLatencyMs(Math.max(0, now - lastEventTimestampRef.current))
    }
    lastEventTimestampRef.current = now
    setLastActivityAt(new Date())
  }, [])

  const handleCommandPaletteToggle = useCallback((keyboardEvent: KeyboardEvent) => {
    if (
      (keyboardEvent.metaKey || keyboardEvent.ctrlKey) &&
      keyboardEvent.key.toLowerCase() === 'k'
    ) {
      keyboardEvent.preventDefault()
      setCommandPaletteOpen((prev) => !prev)
    }
  }, [])

  useEffect(() => {
    window.addEventListener('keydown', handleCommandPaletteToggle)
    return () => window.removeEventListener('keydown', handleCommandPaletteToggle)
  }, [handleCommandPaletteToggle])

  const recordingActive = useMemo(() => {
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
  }, [session?.metadata])

  const commandPaletteOptions = useMemo(() => {
    const tabEntries = tabs.map((tab) => ({
      type: 'tab' as const,
      id: tab.id,
      label: tab.title,
      sessionId,
      tab,
    }))

    const sessionEntries = orderedSessionIds
      .filter((id) => id !== sessionId)
      .map((id) => {
        const record = activeSessions?.find((item) => item.id === id)
        return {
          type: 'session' as const,
          id,
          label: record?.connection_name ?? record?.connection_id ?? id,
          session: record,
        }
      })

    return [...tabEntries, ...sessionEntries]
  }, [tabs, orderedSessionIds, activeSessions, sessionId])

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

  const startedAt = session.started_at ? new Date(session.started_at) : undefined
  const lastSeenAt = session.last_seen_at ? new Date(session.last_seen_at) : undefined

  return (
    <div
      className={cn(
        'flex h-full flex-col gap-6',
        isFullscreen && 'fixed inset-0 z-50 bg-background p-4 lg:p-6'
      )}
    >
      <Card className="border border-border bg-card p-5 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-foreground">
              {session.connection_name ?? 'SSH Session'}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Connected as{' '}
              <span className="font-medium text-foreground">
                {session.user_name ?? session.user_id}
              </span>
              {session.host ? ` · ${session.host}` : ''}
              {session.port ? `:${session.port}` : ''}
            </p>
          </div>
          <dl className="grid grid-cols-2 gap-4 text-sm text-muted-foreground">
            {startedAt && (
              <div>
                <dt className="font-medium text-foreground">Started</dt>
                <dd>{formatDistanceToNow(startedAt, { addSuffix: true })}</dd>
              </div>
            )}
            {lastSeenAt && (
              <div>
                <dt className="font-medium text-foreground">Last activity</dt>
                <dd>{formatDistanceToNow(lastSeenAt, { addSuffix: true })}</dd>
              </div>
            )}
          </dl>
        </div>
      </Card>

      <Tabs
        value={activeTabId}
        onValueChange={handleTabChange}
        className="flex h-full flex-1 flex-col overflow-hidden"
      >
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-background/60 shadow-inner">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 bg-muted/30 px-3 py-2">
            <TabsList className="bg-transparent p-0">
              {tabs.map((tab) => (
                <TabsTrigger
                  key={tab.id}
                  value={tab.id}
                  className="group flex items-center gap-2 px-3 py-1.5"
                  data-testid={`workspace-tab-${tab.type}`}
                >
                  <span>{resolveTabLabel(tab)}</span>
                  {tab.closable && (
                    <span
                      role="button"
                      tabIndex={0}
                      onClick={(event) => handleCloseTab(tab.id, event)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          handleCloseTab(tab.id, event)
                        }
                      }}
                      className="rounded-sm p-0.5 text-muted-foreground outline-none transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring"
                      aria-label={`Close ${tab.title}`}
                    >
                      <X className="h-3 w-3" aria-hidden />
                    </span>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>

            <div className="flex items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" aria-label="Change layout">
                    <LayoutGrid className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuLabel>Layout columns</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {LAYOUT_OPTIONS.map((option) => (
                    <DropdownMenuItem
                      key={option}
                      onSelect={() => handleLayoutSelect(option)}
                      className={cn(
                        'flex items-center justify-between',
                        option === layoutColumns && 'bg-muted text-foreground'
                      )}
                    >
                      <span>
                        {option} column{option > 1 ? 's' : ''}
                      </span>
                      {option === layoutColumns && (
                        <span className="text-xs text-muted-foreground">Active</span>
                      )}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

              <PermissionGuard
                permission={PERMISSIONS.PROTOCOL.SSH.MANAGE_SNIPPETS}
                fallback={null}
              >
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="flex items-center gap-2"
                      disabled={snippetsQuery.isLoading || snippetGroups.length === 0}
                    >
                      <Wand2 className="h-4 w-4" />
                      Snippets
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent className="w-72">
                    {snippetsQuery.isLoading ? (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        Loading snippets…
                      </div>
                    ) : snippetGroups.length === 0 ? (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        No snippets available
                      </div>
                    ) : (
                      snippetGroups.map((group) => (
                        <div key={group.scope}>
                          <DropdownMenuLabel>{group.label}</DropdownMenuLabel>
                          {group.snippets.map((snippet) => (
                            <DropdownMenuItem
                              key={snippet.id}
                              onSelect={() => handleSnippetSelect(snippet)}
                              className="flex flex-col items-start"
                            >
                              <span className="text-sm font-medium">{snippet.name}</span>
                              {snippet.description && (
                                <span className="text-xs text-muted-foreground">
                                  {snippet.description}
                                </span>
                              )}
                            </DropdownMenuItem>
                          ))}
                          <DropdownMenuSeparator />
                        </div>
                      ))
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </PermissionGuard>

              <PermissionGuard permission={PERMISSIONS.PROTOCOL.SSH.SFTP} fallback={null}>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleOpenFileManager}
                  className="flex items-center gap-2"
                >
                  <FileIcon className="h-4 w-4" />
                  File Manager
                </Button>
              </PermissionGuard>

              <Button
                variant="ghost"
                size="sm"
                onClick={handleToggleFullscreen}
                className="flex items-center gap-2"
              >
                {isFullscreen ? (
                  <>
                    <Minimize2 className="h-4 w-4" />
                    Exit Fullscreen
                  </>
                ) : (
                  <>
                    <Maximize2 className="h-4 w-4" />
                    Fullscreen
                  </>
                )}
              </Button>

              <Button
                variant="ghost"
                size="icon"
                onClick={() => setCommandPaletteOpen(true)}
                aria-label="Open command palette"
              >
                <CommandIcon className="h-4 w-4" />
              </Button>

              <Button
                variant="ghost"
                size="icon"
                onClick={handleOpenNewWindow}
                aria-label="Open workspace in new window"
              >
                <ExternalLink className="h-4 w-4" />
              </Button>
            </div>
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
                        onEvent={handleTerminalEvent}
                        onFontSizeChange={setFontSize}
                        searchOverlay={{
                          visible: isSearchOpen,
                          query: searchQuery,
                          direction: searchDirection,
                        }}
                        onSearchResolved={({ matched }) => setSearchMatched(matched)}
                      />
                    </div>
                  ) : (
                    <div className="col-span-full h-full">
                      <SftpWorkspace
                        sessionId={sessionId}
                        canWrite={canWrite}
                        currentUserId={currentUserId}
                        currentUserName={currentUserDisplayName}
                        participants={session.participants}
                      />
                    </div>
                  )}
                </div>
              </TabsContent>
            ))}
          </div>

          <div className="flex flex-col gap-2 border-t border-border/60 bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Button variant="ghost" size="sm" onClick={handleZoomOut} aria-label="Zoom out">
                  -
                </Button>
                <span className="font-mono text-sm">{fontSize}px</span>
                <Button variant="ghost" size="sm" onClick={handleZoomIn} aria-label="Zoom in">
                  +
                </Button>
                <Button variant="ghost" size="sm" onClick={handleZoomReset} aria-label="Reset zoom">
                  Reset
                </Button>
              </div>
              <div className="flex flex-wrap items-center gap-4">
                <button
                  type="button"
                  className={cn(
                    'flex items-center gap-1 rounded-md px-2 py-1 transition-colors',
                    isSearchOpen ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
                  )}
                  onClick={() => {
                    setIsSearchOpen((prev) => !prev)
                    if (!isSearchOpen) {
                      logEvent('terminal.search.open', { sessionId })
                    }
                  }}
                >
                  <SearchIcon className="h-4 w-4" />
                  Search
                </button>
                <span>Latency: {latencyMs != null ? `${Math.round(latencyMs)} ms` : '—'}</span>
                <span>
                  Last activity:{' '}
                  {lastActivityAt ? formatDistanceToNow(lastActivityAt, { addSuffix: true }) : '—'}
                </span>
                <span>
                  Transfers: {transfersSummary.active}/{transfersSummary.total}
                </span>
                {recordingActive && <span className="text-rose-500">Recording</span>}
              </div>
            </div>

            {isSearchOpen && (
              <form className="flex flex-wrap items-center gap-2" onSubmit={handleSearchSubmit}>
                <Input
                  ref={searchInputRef}
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="Search terminal output"
                  className="max-w-xs"
                />
                <div className="flex items-center gap-1">
                  <Button
                    type="button"
                    variant={searchDirection === 'next' ? 'secondary' : 'ghost'}
                    size="sm"
                    onClick={() => setSearchDirection('next')}
                  >
                    Next
                  </Button>
                  <Button
                    type="button"
                    variant={searchDirection === 'previous' ? 'secondary' : 'ghost'}
                    size="sm"
                    onClick={() => setSearchDirection('previous')}
                  >
                    Previous
                  </Button>
                </div>
                <Button type="submit" size="sm" disabled={!searchQuery}>
                  Find
                </Button>
                {!searchMatched && searchQuery && (
                  <span className="text-xs text-rose-500">No matches</span>
                )}
              </form>
            )}
          </div>
        </div>
      </Tabs>

      <Modal
        open={isCommandPaletteOpen}
        onClose={() => setCommandPaletteOpen(false)}
        title="Command palette"
        description="Switch tabs or jump between active sessions."
        size="lg"
      >
        <div className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-foreground">Current workspace</h3>
            <div className="mt-2 grid gap-2">
              {tabs.map((tab) => (
                <Button
                  key={tab.id}
                  variant={tab.id === activeTabId ? 'secondary' : 'outline'}
                  className="justify-start"
                  onClick={() => {
                    setActiveTab(sessionId, tab.id)
                    setCommandPaletteOpen(false)
                    logEvent('palette.switch_tab', { tabId: tab.id })
                  }}
                >
                  <span className="font-medium">{tab.title}</span>
                  {tab.id === activeTabId && (
                    <span className="ml-auto text-xs text-muted-foreground">Active</span>
                  )}
                </Button>
              ))}
            </div>
          </div>

          {commandPaletteOptions.filter((item) => item.type === 'session').length > 0 && (
            <div>
              <h3 className="text-sm font-semibold text-foreground">Other sessions</h3>
              <div className="mt-2 grid gap-2">
                {commandPaletteOptions
                  .filter((item) => item.type === 'session')
                  .map((item) => (
                    <Button
                      key={item.id}
                      variant="outline"
                      className="justify-start"
                      onClick={() => {
                        setCommandPaletteOpen(false)
                        navigate(`/active-sessions/${item.id}`)
                        logEvent('palette.switch_session', { target: item.id })
                      }}
                    >
                      {item.label}
                    </Button>
                  ))}
              </div>
            </div>
          )}
        </div>
      </Modal>
    </div>
  )
}

export default SshWorkspace
