import type { LucideIcon } from 'lucide-react'
import {
  Activity,
  Building2,
  Cloud,
  Container,
  Database,
  FileText,
  FolderKanban,
  HardDrive,
  Key,
  LayoutDashboard,
  Monitor,
  Network,
  Server,
  Settings,
  Shield,
  Users,
} from 'lucide-react'

export interface NavigationItem {
  label: string
  path: string
  icon?: LucideIcon
  permission?: string
  children?: NavigationItem[]
  exact?: boolean
  group?: string
}

export interface NavigationGroup {
  label: string
  items: NavigationItem[]
}

export const NAVIGATION_GROUPS: NavigationGroup[] = [
  {
    label: 'Main',
    items: [
      { label: 'Dashboard', path: '/dashboard', icon: LayoutDashboard },
      {
        label: 'Connections',
        path: '/connections',
        icon: FolderKanban,
        permission: 'connection.view',
        children: [
          { label: 'All Connections', path: '/connections' },
          { label: 'Folders', path: '/connections/folders', permission: 'connection.folder.view' },
          { label: 'New Connection', path: '/connections/new' },
        ],
      },
    ],
  },
  {
    label: 'Protocol Catalog',
    items: [
      { label: 'SSH / Telnet', path: '/ssh', icon: Server },
      { label: 'RDP', path: '/rdp', icon: Monitor },
      { label: 'VNC', path: '/vnc', icon: Monitor },
      { label: 'Docker', path: '/docker', icon: Container },
      { label: 'Kubernetes', path: '/kubernetes', icon: Cloud },
      { label: 'Databases', path: '/databases', icon: Database },
      { label: 'File Share', path: '/file-share', icon: HardDrive },
      { label: 'Proxmox', path: '/proxmox', icon: HardDrive },
      { label: 'Network Devices', path: '/network', icon: Network },
    ],
  },
  {
    label: 'Settings',
    items: [
      { label: 'Identities', path: '/settings/identities', icon: Key },
      { label: 'Users', path: '/settings/users', icon: Users, permission: 'user.view' },
      {
        label: 'Organizations',
        path: '/settings/organizations',
        icon: Building2,
        permission: 'org.view',
      },
      { label: 'Teams', path: '/settings/teams', icon: Users, permission: 'org.view' },
      {
        label: 'Permissions',
        path: '/settings/permissions',
        icon: Shield,
        permission: 'permission.view',
      },
      {
        label: 'Auth Providers',
        path: '/settings/auth-providers',
        icon: Key,
        permission: 'permission.manage',
      },
      { label: 'Sessions', path: '/settings/sessions', icon: Activity },
      {
        label: 'Audit Logs',
        path: '/settings/audit',
        icon: FileText,
        permission: 'audit.view',
      },
      { label: 'Security', path: '/settings/security', icon: Settings },
    ],
  },
]

function flattenNavigation(items: NavigationItem[], groupLabel?: string): NavigationItem[] {
  return items.flatMap((item) => {
    const enriched: NavigationItem = {
      ...item,
      group: groupLabel ?? item.group,
    }
    if (item.children?.length) {
      return [enriched, ...flattenNavigation(item.children, groupLabel ?? item.label)]
    }
    return [enriched]
  })
}

export const NAVIGATION_ITEMS: NavigationItem[] = NAVIGATION_GROUPS.flatMap((group) =>
  flattenNavigation(group.items, group.label)
)

export function findNavigationItem(pathname: string): NavigationItem | undefined {
  return (
    NAVIGATION_ITEMS.find((item) => item.path === pathname) ??
    NAVIGATION_ITEMS.find(
      (item) =>
        item.path !== '/' &&
        pathname.startsWith(item.path.endsWith('/') ? item.path : `${item.path}/`)
    )
  )
}

export function getBreadcrumbItems(pathname: string): NavigationItem[] {
  const segments = pathname.split('/').filter(Boolean)

  if (!segments.length) {
    return []
  }

  const crumbs: NavigationItem[] = []
  let currentPath = ''

  segments.forEach((segment) => {
    currentPath = `${currentPath}/${segment}`
    const match = findNavigationItem(currentPath)

    if (match) {
      crumbs.push(match)
    } else {
      crumbs.push({
        label: segment.replace(/-/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase()),
        path: currentPath,
      })
    }
  })

  return crumbs
}
