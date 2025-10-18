import { useEffect, useMemo, useRef, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import {
  ChevronDown,
  ChevronRight,
  Loader2,
  MoreHorizontal,
  Settings as SettingsIcon,
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { APP_NAME } from '@/lib/constants'
import { Logo } from '@/components/ui/Logo'
import { Badge } from '@/components/ui/Badge'
import { getFilteredNavigationGroups, type NavigationItem } from '@/lib/navigation'
import { Collapsible } from '@/components/ui/Collapsible'
import { cn } from '@/lib/utils/cn'
import { usePermissions } from '@/hooks/usePermissions'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { PERMISSIONS } from '@/constants/permissions'
import { useSshWorkspaceTabsStore } from '@/store/ssh-session-tabs-store'
import { useLaunchConnectionContext } from '@/contexts/LaunchConnectionContext'
import { getWorkspaceDescriptor } from '@/workspaces/protocolWorkspaceRegistry'

interface SidebarProps {
  isOpen?: boolean
  onClose?: () => void
}

export function Sidebar({ isOpen = false, onClose }: SidebarProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { hasPermission, hasAnyPermission, hasAllPermissions } = usePermissions()
  const launchContext = useLaunchConnectionContext()
  const openSession = useSshWorkspaceTabsStore((state) => state.openSession)
  const ensureTab = useSshWorkspaceTabsStore((state) => state.ensureTab)
  const focusSession = useSshWorkspaceTabsStore((state) => state.focusSession)
  const setActiveTab = useSshWorkspaceTabsStore((state) => state.setActiveTab)

  const canViewConnections = hasAnyPermission([
    PERMISSIONS.CONNECTION.VIEW,
    PERMISSIONS.CONNECTION.VIEW_ALL,
    PERMISSIONS.CONNECTION.MANAGE,
    PERMISSIONS.PERMISSION.MANAGE,
  ])
  const isAdmin = hasAnyPermission([
    PERMISSIONS.PERMISSION.MANAGE,
    PERMISSIONS.CONNECTION.VIEW_ALL,
    PERMISSIONS.CONNECTION.MANAGE,
  ])

  const { data: activeSessions = [], isLoading: activeSessionsLoading } = useActiveConnections({
    enabled: canViewConnections,
    refetchInterval: 10_000,
  })

  const activeConnections = useMemo(() => {
    if (!activeSessions?.length) {
      return []
    }

    const grouped = new Map<
      string,
      {
        connectionId: string
        connectionName: string
        sessions: typeof activeSessions
      }
    >()

    activeSessions.forEach((session) => {
      const key = session.connection_id
      const resolvedName =
        (session.connection_name && session.connection_name.trim()) ||
        (session.host && session.host.trim()) ||
        key
      const existing = grouped.get(key)
      if (existing) {
        existing.sessions.push(session)
        if (!existing.connectionName && resolvedName) {
          existing.connectionName = resolvedName
        }
      } else {
        grouped.set(key, {
          connectionId: key,
          connectionName: resolvedName,
          sessions: [session],
        })
      }
    })

    const toTimestamp = (value: string | null) => {
      if (!value) {
        return 0
      }
      const millis = new Date(value).getTime()
      return Number.isNaN(millis) ? 0 : millis
    }

    const items = Array.from(grouped.values()).map((entry) => {
      entry.sessions.sort(
        (a, b) => toTimestamp(b.last_seen_at ?? null) - toTimestamp(a.last_seen_at ?? null)
      )
      const latest = entry.sessions[0]?.last_seen_at ?? null
      return {
        connectionId: entry.connectionId,
        connectionName: entry.connectionName || entry.connectionId,
        sessions: entry.sessions,
        sessionCount: entry.sessions.length,
        lastSeenAt: latest,
      }
    })

    items.sort((a, b) => toTimestamp(b.lastSeenAt) - toTimestamp(a.lastSeenAt))
    return items
  }, [activeSessions])

  const navigationGroups = useMemo(
    () => getFilteredNavigationGroups({ hasPermission }),
    [hasPermission]
  )

  const settingsGroup = useMemo(
    () => navigationGroups.find((group) => group.label === 'Settings'),
    [navigationGroups]
  )
  const otherGroups = useMemo(
    () => navigationGroups.filter((group) => group.label !== 'Settings'),
    [navigationGroups]
  )
  const visibleSettingsItems = useMemo(() => {
    if (!settingsGroup) {
      return []
    }
    return settingsGroup.items.filter((item) => {
      if (item.permission && !hasPermission(item.permission)) {
        return false
      }
      if (item.allPermissions?.length && !hasAllPermissions(item.allPermissions)) {
        return false
      }
      if (item.anyPermissions?.length && !hasAnyPermission(item.anyPermissions)) {
        return false
      }
      return true
    })
  }, [settingsGroup, hasPermission, hasAllPermissions, hasAnyPermission])

  const isSettingsRouteActive = useMemo(() => {
    const currentPath = location.pathname

    return visibleSettingsItems.some((item) => {
      if (item.exact) {
        return currentPath === item.path
      }

      const normalized = item.path.endsWith('/') ? item.path : `${item.path}/`
      return currentPath === item.path || currentPath.startsWith(normalized)
    })
  }, [visibleSettingsItems, location.pathname])

  const wasSettingsRouteActiveRef = useRef(isSettingsRouteActive)
  const [settingsOpen, setSettingsOpen] = useState(isSettingsRouteActive)

  useEffect(() => {
    if (isSettingsRouteActive) {
      setSettingsOpen(true)
    } else if (wasSettingsRouteActiveRef.current) {
      setSettingsOpen(false)
    }

    wasSettingsRouteActiveRef.current = isSettingsRouteActive
  }, [isSettingsRouteActive])

  const navContent = (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center justify-between border-b border-border px-4">
        <div className="flex items-center gap-2">
          <Logo size="md" />
          <span className="text-sm font-semibold tracking-wide">{APP_NAME}</span>
        </div>
        <button
          type="button"
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground lg:hidden"
          onClick={onClose}
          aria-label="Close navigation"
        >
          <MoreHorizontal className="h-5 w-5" />
        </button>
      </div>

      <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-4 scrollbar-thin">
        {otherGroups.map((group) => (
          <NavSection
            key={group.label}
            label={group.label}
            items={group.items}
            activePath={location.pathname}
          />
        ))}

        {canViewConnections ? (
          <div className="space-y-2">
            <div className="px-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Active Sessions
            </div>
            {activeSessionsLoading ? (
              <div className="flex items-center gap-2 rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Tracking sessions...
              </div>
            ) : activeConnections.length === 0 ? (
              <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-center text-xs text-muted-foreground">
                No active sessions
              </div>
            ) : (
              <div className="space-y-1">
                {activeConnections.map((item) => {
                  const latestSeen = item.lastSeenAt ? new Date(item.lastSeenAt) : null
                  const latestSeenLabel =
                    latestSeen && !Number.isNaN(latestSeen.getTime())
                      ? formatDistanceToNow(latestSeen, { addSuffix: true })
                      : null
                  const adminTooltip = isAdmin
                    ? item.sessions
                        .map((session) => {
                          const userName = session.user_name?.trim() || session.user_id
                          const startedAt = session.started_at ? new Date(session.started_at) : null
                          const startedLabel =
                            startedAt && !Number.isNaN(startedAt.getTime())
                              ? formatDistanceToNow(startedAt, { addSuffix: true })
                              : null
                          return startedLabel ? `${userName} • ${startedLabel}` : userName
                        })
                        .join('\n')
                    : undefined

                  return (
                    <div key={item.connectionId} className="rounded-md border border-border/40 p-2">
                      <button
                        type="button"
                        onClick={() => {
                          void launchContext.openById(item.connectionId)
                          onClose?.()
                        }}
                        className="flex w-full items-center justify-between rounded-md px-2 py-1 text-left text-sm font-medium text-foreground transition hover:bg-muted"
                        title={adminTooltip ?? latestSeenLabel ?? undefined}
                      >
                        <span className="truncate">{item.connectionName}</span>
                        <span className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
                          {item.sessionCount}
                          {latestSeenLabel && (
                            <Badge variant="outline" className="text-[10px] font-semibold">
                              {latestSeenLabel}
                            </Badge>
                          )}
                        </span>
                      </button>

                      <div className="mt-2 space-y-1">
                        {item.sessions.map((sessionRecord) => {
                          const sessionActive = location.pathname.startsWith(
                            `/active-sessions/${sessionRecord.id}`
                          )
                          return (
                            <button
                              key={sessionRecord.id}
                              type="button"
                              className={cn(
                                'w-full rounded-md px-2 py-1 text-left text-xs transition',
                                sessionActive
                                  ? 'bg-primary text-primary-foreground shadow'
                                  : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                              )}
                              onClick={() => {
                                const descriptorForSession = getWorkspaceDescriptor(
                                  sessionRecord.descriptor_id ?? sessionRecord.protocol_id
                                )
                                openSession({
                                  sessionId: sessionRecord.id,
                                  connectionId: sessionRecord.connection_id,
                                  connectionName: sessionRecord.connection_name,
                                })
                                const terminalTab = ensureTab(sessionRecord.id, 'terminal', {
                                  title: 'Terminal',
                                  closable: false,
                                })
                                focusSession(sessionRecord.id)
                                setActiveTab(sessionRecord.id, terminalTab.id)
                                navigate(descriptorForSession.defaultRoute(sessionRecord.id))
                                onClose?.()
                              }}
                            >
                              <span className="block truncate font-medium">
                                {sessionRecord.user_name?.trim() || sessionRecord.user_id}
                              </span>
                              <span className="block truncate text-[10px] text-muted-foreground">
                                {sessionRecord.connection_name ??
                                  sessionRecord.host ??
                                  sessionRecord.connection_id}
                              </span>
                            </button>
                          )
                        })}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        ) : null}

        {settingsGroup && visibleSettingsItems.length ? (
          <div>
            <button
              type="button"
              onClick={() => {
                setSettingsOpen((open) => !open)
              }}
              className="flex w-full items-center justify-between rounded-md px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground hover:bg-muted/60"
            >
              <span className="inline-flex items-center gap-2">
                <SettingsIcon className="h-4 w-4" />
                Settings
              </span>
              {settingsOpen ? (
                <ChevronDown className="h-4 w-4" />
              ) : (
                <ChevronRight className="h-4 w-4" />
              )}
            </button>

            <Collapsible isOpen={settingsOpen} className="mt-2">
              <div className="space-y-1">
                <NavItems items={visibleSettingsItems} activePath={location.pathname} />
              </div>
            </Collapsible>
          </div>
        ) : null}
      </nav>
    </div>
  )

  return (
    <>
      <aside className="fixed left-0 top-0 z-40 hidden h-screen w-64 flex-col border-r border-border bg-background lg:flex">
        {navContent}
      </aside>
      <div className="lg:hidden">
        <div
          className={cn(
            'fixed inset-0 z-40 bg-background/70 backdrop-blur-sm transition-opacity duration-200',
            isOpen ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0'
          )}
          onClick={isOpen ? onClose : undefined}
          aria-hidden={!isOpen}
        />
        <aside
          className={cn(
            'fixed inset-y-0 left-0 z-50 h-full w-64 border-r border-border bg-background shadow-xl transition-transform duration-200 ease-out will-change-transform',
            isOpen ? 'translate-x-0' : '-translate-x-full'
          )}
          aria-hidden={!isOpen}
        >
          {navContent}
        </aside>
      </div>
    </>
  )
}

interface NavSectionProps {
  label: string
  items: NavigationItem[]
  activePath: string
}

function NavSection({ label, items, activePath }: NavSectionProps) {
  const filtered = useMemo(() => items, [items])

  if (!filtered.length) {
    return null
  }

  return (
    <div className="space-y-1">
      <h4 className="px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {label}
      </h4>
      <NavItems items={filtered} activePath={activePath} />
    </div>
  )
}

interface NavItemsProps {
  items: NavigationItem[]
  activePath: string
}

function NavItems({ items, activePath }: NavItemsProps) {
  const { hasPermission, hasAnyPermission, hasAllPermissions } = usePermissions()

  return (
    <>
      {items.map((item) => {
        if (item.permission && !hasPermission(item.permission)) {
          return null
        }
        if (item.allPermissions?.length && !hasAllPermissions(item.allPermissions)) {
          return null
        }
        if (item.anyPermissions?.length && !hasAnyPermission(item.anyPermissions)) {
          return null
        }

        return (
          <PermissionGuard
            key={item.path}
            permission={item.permission}
            anyOf={item.anyPermissions}
            allOf={item.allPermissions}
          >
            <NavLink
              to={item.path}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition',
                  isActive || (!item.exact && activePath.startsWith(item.path))
                    ? 'bg-primary text-primary-foreground shadow'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )
              }
            >
              {item.icon ? <item.icon className="h-4 w-4" /> : null}
              <span className="truncate">{item.label}</span>
            </NavLink>
          </PermissionGuard>
        )
      })}
    </>
  )
}
