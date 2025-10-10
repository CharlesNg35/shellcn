import type { LucideIcon } from 'lucide-react'
import {
  Activity,
  FileText,
  FolderKanban,
  Key,
  LayoutDashboard,
  Settings,
  Shield,
  Users,
} from 'lucide-react'
import { isFeatureEnabled } from './features'

export interface NavigationItem {
  label: string
  path: string
  icon?: LucideIcon
  permission?: string
  children?: NavigationItem[]
  exact?: boolean
  group?: string
  featureId?: string // Feature module ID for toggling
}

export interface NavigationGroup {
  label: string
  items: NavigationItem[]
}

export const NAVIGATION_GROUPS: NavigationGroup[] = [
  {
    label: 'Main',
    items: [
      { label: 'Dashboard', path: '/dashboard', icon: LayoutDashboard, featureId: 'dashboard' },
      {
        label: 'Connections',
        path: '/connections',
        icon: FolderKanban,
        permission: 'connection.view',
        featureId: 'connections',
      },
    ],
  },
  {
    label: 'Settings',
    items: [
      { label: 'Identities', path: '/settings/identities', icon: Key, featureId: 'identities' },
      {
        label: 'Users',
        path: '/settings/users',
        icon: Users,
        permission: 'user.view',
        featureId: 'users',
      },
      {
        label: 'Teams',
        path: '/settings/teams',
        icon: Users,
        featureId: 'teams',
      },
      {
        label: 'Permissions',
        path: '/settings/permissions',
        icon: Shield,
        permission: 'permission.view',
        featureId: 'permissions',
      },
      {
        label: 'Auth Providers',
        path: '/settings/auth-providers',
        icon: Key,
        permission: 'permission.manage',
        featureId: 'authProviders',
      },
      { label: 'Sessions', path: '/settings/sessions', icon: Activity, featureId: 'sessions' },
      {
        label: 'Audit Logs',
        path: '/settings/audit',
        icon: FileText,
        permission: 'audit.view',
        featureId: 'auditLogs',
      },
      { label: 'Security', path: '/settings/security', icon: Settings, featureId: 'security' },
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

/**
 * Filter navigation groups by enabled features
 */
export function getFilteredNavigationGroups(): NavigationGroup[] {
  return NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      // If item has a featureId, check if it's enabled
      if (item.featureId && !isFeatureEnabled(item.featureId)) {
        return false
      }
      return true
    }),
  })).filter((group) => group.items.length > 0) // Remove empty groups
}
