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
import { PERMISSIONS } from '@/constants/permissions'
import type { PermissionId } from '@/constants/permissions'

export interface NavigationItem {
  label: string
  path: string
  icon?: LucideIcon
  permission?: PermissionId
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
        permission: PERMISSIONS.CONNECTION.VIEW,
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
        permission: PERMISSIONS.USER.VIEW,
        featureId: 'users',
      },
      {
        label: 'Teams',
        path: '/settings/teams',
        icon: Users,
        featureId: 'teams',
        permission: PERMISSIONS.TEAM.VIEW,
      },
      {
        label: 'Permissions',
        path: '/settings/permissions',
        icon: Shield,
        permission: PERMISSIONS.PERMISSION.VIEW,
        featureId: 'permissions',
      },
      {
        label: 'Auth Providers',
        path: '/settings/auth-providers',
        icon: Key,
        permission: PERMISSIONS.PERMISSION.MANAGE,
        featureId: 'authProviders',
      },
      { label: 'Sessions', path: '/settings/sessions', icon: Activity, featureId: 'sessions' },
      {
        label: 'Audit Logs',
        path: '/settings/audit',
        icon: FileText,
        permission: PERMISSIONS.AUDIT.VIEW,
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
    const exactMatch = NAVIGATION_ITEMS.find((item) => item.path === currentPath)

    if (exactMatch) {
      // Found an exact match in navigation
      crumbs.push(exactMatch)
    } else {
      // Check if this is a detail/child page of a parent route
      const parentMatch = NAVIGATION_ITEMS.find(
        (item) =>
          item.path !== '/' &&
          currentPath.startsWith(item.path.endsWith('/') ? item.path : `${item.path}/`)
      )

      if (parentMatch && crumbs[crumbs.length - 1]?.path === parentMatch.path) {
        // Parent is already in breadcrumbs, add this as a child
        crumbs.push({
          label: segment.replace(/-/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase()),
          path: currentPath,
        })
      } else {
        // No parent in breadcrumbs yet, add as regular item
        crumbs.push({
          label: segment.replace(/-/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase()),
          path: currentPath,
        })
      }
    }
  })

  return crumbs
}

/**
 * Filter navigation groups by enabled features
 */
interface NavigationFilterOptions {
  hasPermission?: (permission: PermissionId) => boolean
}

export function getFilteredNavigationGroups(
  options: NavigationFilterOptions = {}
): NavigationGroup[] {
  const { hasPermission } = options

  return NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      // If item has a featureId, check if it's enabled
      if (item.featureId && !isFeatureEnabled(item.featureId)) {
        return false
      }

      if (item.permission && hasPermission) {
        return hasPermission(item.permission)
      }

      return true
    }),
  })).filter((group) => group.items.length > 0) // Remove empty groups
}
