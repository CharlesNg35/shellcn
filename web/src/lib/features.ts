import { PERMISSIONS } from '@/constants/permissions'
import type { PermissionId } from '@/constants/permissions'

/**
 * Feature flags and module configuration
 * Controls which core features and settings pages are enabled
 *
 * Note: Protocols (SSH, Docker, K8s, etc.) are NOT features - they are connection types
 * managed through the backend ProtocolService. Protocol availability is determined by:
 * - Backend protocol drivers (what's installed)
 * - User permissions (user.permission check)
 * All protocols are accessed via /connections page
 */

export interface FeatureModule {
  id: string
  name: string
  enabled: boolean
  permission?: PermissionId
  description?: string
  category?: 'core' | 'settings'
}

export const FEATURE_MODULES: Record<string, FeatureModule> = {
  // Core application features
  dashboard: {
    id: 'dashboard',
    name: 'Dashboard',
    enabled: true,
    category: 'core',
    description: 'Main dashboard with overview and statistics',
  },
  connections: {
    id: 'connections',
    name: 'Connections',
    enabled: true,
    category: 'core',
    permission: PERMISSIONS.CONNECTION.VIEW,
    description: 'Unified connection management for all protocols',
  },
  identities: {
    id: 'identities',
    name: 'Identities',
    enabled: true,
    category: 'core',
    description: 'Credential vault and identity management',
  },

  // Settings & Administration features
  users: {
    id: 'users',
    name: 'Users',
    enabled: true,
    category: 'settings',
    permission: PERMISSIONS.USER.VIEW,
    description: 'User management and administration',
  },
  teams: {
    id: 'teams',
    name: 'Teams',
    enabled: true,
    category: 'settings',
    permission: PERMISSIONS.ORGANIZATION.VIEW,
    description: 'Team management and user grouping',
  },
  permissions: {
    id: 'permissions',
    name: 'Permissions',
    enabled: true,
    category: 'settings',
    permission: PERMISSIONS.PERMISSION.VIEW,
    description: 'Permission and role management',
  },
  authProviders: {
    id: 'authProviders',
    name: 'Auth Providers',
    enabled: true,
    category: 'settings',
    permission: PERMISSIONS.PERMISSION.MANAGE,
    description: 'SSO and authentication provider configuration',
  },
  sessions: {
    id: 'sessions',
    name: 'Sessions',
    enabled: true,
    category: 'settings',
    description: 'Active session monitoring and management',
  },
  auditLogs: {
    id: 'auditLogs',
    name: 'Audit Logs',
    enabled: true,
    category: 'settings',
    permission: PERMISSIONS.AUDIT.VIEW,
    description: 'Security audit trail and compliance logs',
  },
  security: {
    id: 'security',
    name: 'Security',
    enabled: true,
    category: 'settings',
    description: 'Security settings, MFA, and policies',
  },
}

/**
 * Check if a feature module is enabled
 */
export function isFeatureEnabled(moduleId: string): boolean {
  const module = FEATURE_MODULES[moduleId]
  return module?.enabled ?? false
}

/**
 * Get all enabled modules by category
 */
export function getEnabledModules(category?: string): FeatureModule[] {
  return Object.values(FEATURE_MODULES).filter(
    (module) => module.enabled && (!category || module.category === category)
  )
}

/**
 * Get all enabled core modules
 */
export function getEnabledCoreModules(): FeatureModule[] {
  return getEnabledModules('core')
}

/**
 * Get all enabled settings modules
 */
export function getEnabledSettings(): FeatureModule[] {
  return getEnabledModules('settings')
}

/**
 * Filter navigation items based on enabled features
 */
export function filterNavigationByFeatures<T extends { path: string }>(items: T[]): T[] {
  return items.filter((item) => {
    // Extract module ID from path (e.g., /dashboard -> dashboard, /settings/users -> users)
    const pathSegments = item.path.split('/').filter(Boolean)
    const moduleId = pathSegments[pathSegments.length - 1]

    // If no matching module, allow by default
    if (!FEATURE_MODULES[moduleId]) {
      return true
    }

    return isFeatureEnabled(moduleId)
  })
}
