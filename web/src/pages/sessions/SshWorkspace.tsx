import { useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { LayoutGrid, Loader2, X } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { selectSessionWorkspace, useSshWorkspaceTabsStore } from '@/store/ssh-session-tabs-store'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import { SftpWorkspace } from '@/components/workspace/SftpWorkspace'
import { SshTerminal } from '@/components/workspace/SshTerminal'
import { Button } from '@/components/ui/Button'

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

export function SshWorkspace() {
  const { sessionId = '' } = useParams<{ sessionId: string }>()
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
  const focusSession = useSshWorkspaceTabsStore((state) => state.focusSession)
  const setLayoutColumns = useSshWorkspaceTabsStore((state) => state.setLayoutColumns)

  const workspace = useSshWorkspaceTabsStore(selectSessionWorkspace(sessionId))
  const layoutColumns = workspace?.layoutColumns ?? 1

  useEffect(() => {
    if (!session) {
      return
    }
    openSession({
      sessionId: session.id,
      connectionId: session.connection_id,
      connectionName: session.connection_name,
    })
    ensureTab(session.id, 'terminal', { title: 'Terminal', closable: false })
    ensureTab(session.id, 'sftp', { title: 'Files', closable: true })
    focusSession(session.id)
  }, [ensureTab, focusSession, openSession, session])

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

  const activeTabId = workspace?.activeTabId ?? ''
  const tabs = workspace?.tabs ?? []
  const layoutOptions = [1, 2, 3, 4, 5]

  const handleSelectColumns = (columns: number) => {
    if (!sessionId) {
      return
    }
    setLayoutColumns(sessionId, columns)
  }

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
    <div className="flex h-full flex-col gap-6">
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
        <TabsList className="mb-4 w-fit bg-muted/40 px-2 py-1">
          {tabs.map((tab) => (
            <TabsTrigger
              key={tab.id}
              value={tab.id}
              className="group flex items-center gap-2"
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

        <div className="flex-1 overflow-hidden rounded-xl border border-border bg-background/50 shadow-inner">
          {tabs.map((tab) => (
            <TabsContent
              key={tab.id}
              value={tab.id}
              className="h-full w-full rounded-b-xl border-0 bg-transparent p-0"
              forceMount
            >
              <div className="flex h-full w-full flex-col gap-4 p-4">
                <div className="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground shadow-sm">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <LayoutGrid className="h-4 w-4" aria-hidden />
                    <span>Layout</span>
                  </div>
                  <div className="flex items-center gap-1">
                    {layoutOptions.map((option) => {
                      const isActive = option === layoutColumns
                      return (
                        <Button
                          key={option}
                          variant={isActive ? 'secondary' : 'ghost'}
                          size="sm"
                          onClick={() => handleSelectColumns(option)}
                          aria-pressed={isActive}
                          aria-label={`${option} column${option > 1 ? 's' : ''}`}
                          title={`${option} column${option > 1 ? 's' : ''}`}
                          className="min-w-8 px-0"
                        >
                          {option}
                        </Button>
                      )
                    })}
                  </div>
                </div>

                <div
                  className="grid h-full gap-4"
                  style={{ gridTemplateColumns: `repeat(${layoutColumns}, minmax(0, 1fr))` }}
                  data-columns={layoutColumns}
                  data-testid={tab.type === 'terminal' ? 'terminal-grid' : undefined}
                >
                  {tab.type === 'terminal' ? (
                    <div className="col-span-full h-full">
                      <SshTerminal sessionId={sessionId} />
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
              </div>
            </TabsContent>
          ))}
        </div>
      </Tabs>
    </div>
  )
}

export default SshWorkspace
