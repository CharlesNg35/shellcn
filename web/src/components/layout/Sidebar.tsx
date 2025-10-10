import { useMemo, useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import {
  BadgePlus,
  ChevronDown,
  ChevronRight,
  Loader2,
  MoreHorizontal,
  Settings as SettingsIcon,
} from 'lucide-react'
import { APP_NAME } from '@/lib/constants'
import { Logo } from '@/components/ui/Logo'
import { getFilteredNavigationGroups, type NavigationItem } from '@/lib/navigation'
import { cn } from '@/lib/utils/cn'
import { usePermissions } from '@/hooks/usePermissions'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { useConnectionFolders } from '@/hooks/useConnectionFolders'
import { FolderTree } from '@/components/connections/FolderTree'

interface SidebarProps {
  isOpen?: boolean
  onClose?: () => void
}

export function Sidebar({ isOpen = false, onClose }: SidebarProps) {
  const location = useLocation()
  const { hasPermission } = usePermissions()
  const searchParams = useMemo(() => new URLSearchParams(location.search), [location.search])
  const activeFolderId =
    searchParams.get('folder') ?? (searchParams.get('view') === 'unassigned' ? 'unassigned' : null)
  const { data: folderTree, isLoading: foldersLoading } = useConnectionFolders({
    enabled: hasPermission('connection.folder.view'),
  })

  const [settingsOpen, setSettingsOpen] = useState(false)

  const navigationGroups = useMemo(() => getFilteredNavigationGroups(), [])

  const settingsGroup = useMemo(
    () => navigationGroups.find((group) => group.label === 'Settings'),
    [navigationGroups]
  )
  const otherGroups = useMemo(
    () => navigationGroups.filter((group) => group.label !== 'Settings'),
    [navigationGroups]
  )

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

        {hasPermission('connection.folder.view') ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between px-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              <span>Connection Folders</span>
              <Link
                to="/connections/new"
                className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground hover:text-foreground"
              >
                <BadgePlus className="h-3 w-3" />
                New
              </Link>
            </div>
            {foldersLoading ? (
              <div className="flex items-center gap-2 rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading folders...
              </div>
            ) : (
              <FolderTree
                nodes={folderTree}
                activeFolderId={activeFolderId}
                basePath="/connections"
              />
            )}
          </div>
        ) : null}

        {settingsGroup ? (
          <div>
            <button
              type="button"
              onClick={() => setSettingsOpen((open) => !open)}
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
            {settingsOpen ? (
              <div className="mt-2 space-y-1">
                <NavItems items={settingsGroup.items} activePath={location.pathname} />
              </div>
            ) : null}
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
      <div
        className={cn(
          'lg:hidden',
          isOpen ? 'fixed inset-0 z-50 flex' : 'pointer-events-none fixed inset-0 z-50 hidden'
        )}
      >
        <div className="fixed inset-0 bg-background/70 backdrop-blur-sm" onClick={onClose} />
        <aside className="relative h-full w-64 border-r border-border bg-background shadow-xl">
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
  const { hasPermission } = usePermissions()

  return (
    <>
      {items.map((item) => {
        if (item.permission && !hasPermission(item.permission)) {
          return null
        }

        return (
          <PermissionGuard key={item.path} permission={item.permission}>
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
