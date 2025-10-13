const HUMANIZED_BASE_MODULES: Record<string, string> = {
  core: 'Core Platform',
  connection: 'Connections',
  user: 'User Management',
  team: 'Team Management',
  permission: 'Permission Management',
  audit: 'Audit & Compliance',
  security: 'Security',
  notification: 'Notifications',
  vault: 'Credential Vault',
  org: 'Organization',
  admin: 'Administration',
}

function toTitleCase(value: string): string {
  return value
    .split(/[-_]/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

export function humanizePermissionModule(moduleId?: string | null): string {
  if (!moduleId) {
    return 'General'
  }

  const trimmed = moduleId.trim()
  if (HUMANIZED_BASE_MODULES[trimmed]) {
    return HUMANIZED_BASE_MODULES[trimmed]
  }

  if (trimmed.startsWith('protocols.')) {
    const [, driverId] = trimmed.split('.', 2)
    if (driverId) {
      return `Protocol • ${toTitleCase(driverId)}`
    }
    return 'Protocols'
  }

  return trimmed.replace(/\./g, ' › ')
}
