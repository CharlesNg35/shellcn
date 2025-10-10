export const PERMISSIONS = {
  USER: {
    VIEW: 'user.view',
    CREATE: 'user.create',
    EDIT: 'user.edit',
    DELETE: 'user.delete',
  },
  TEAM: {
    VIEW: 'team.view',
    MANAGE: 'team.manage',
  },
  CONNECTION: {
    VIEW: 'connection.view',
    LAUNCH: 'connection.launch',
    MANAGE: 'connection.manage',
    SHARE: 'connection.share',
  },
  CONNECTION_FOLDER: {
    VIEW: 'connection.folder.view',
    MANAGE: 'connection.folder.manage',
  },
  VAULT: {
    VIEW: 'vault.view',
    CREATE: 'vault.create',
    EDIT: 'vault.edit',
    DELETE: 'vault.delete',
    SHARE: 'vault.share',
    USE_SHARED: 'vault.use_shared',
    MANAGE_ALL: 'vault.manage_all',
  },
  PERMISSION: {
    VIEW: 'permission.view',
    MANAGE: 'permission.manage',
  },
  AUDIT: {
    VIEW: 'audit.view',
    EXPORT: 'audit.export',
  },
  SECURITY: {
    AUDIT: 'security.audit',
  },
  ORGANIZATION: {
    VIEW: 'org.view',
    MANAGE: 'org.manage',
  },
  NOTIFICATION: {
    VIEW: 'notification.view',
    MANAGE: 'notification.manage',
  },
} as const

type PermissionTree = typeof PERMISSIONS

type PermissionValues<T> =
  T extends Record<string, infer V> ? (V extends string ? V : PermissionValues<V>) : never

export type PermissionId = PermissionValues<PermissionTree>

export const PERMISSION_IDS = [
  PERMISSIONS.USER.VIEW,
  PERMISSIONS.USER.CREATE,
  PERMISSIONS.USER.EDIT,
  PERMISSIONS.USER.DELETE,
  PERMISSIONS.TEAM.VIEW,
  PERMISSIONS.TEAM.MANAGE,
  PERMISSIONS.CONNECTION.VIEW,
  PERMISSIONS.CONNECTION.LAUNCH,
  PERMISSIONS.CONNECTION.MANAGE,
  PERMISSIONS.CONNECTION.SHARE,
  PERMISSIONS.CONNECTION_FOLDER.VIEW,
  PERMISSIONS.CONNECTION_FOLDER.MANAGE,
  PERMISSIONS.VAULT.VIEW,
  PERMISSIONS.VAULT.CREATE,
  PERMISSIONS.VAULT.EDIT,
  PERMISSIONS.VAULT.DELETE,
  PERMISSIONS.VAULT.SHARE,
  PERMISSIONS.VAULT.USE_SHARED,
  PERMISSIONS.VAULT.MANAGE_ALL,
  PERMISSIONS.PERMISSION.VIEW,
  PERMISSIONS.PERMISSION.MANAGE,
  PERMISSIONS.AUDIT.VIEW,
  PERMISSIONS.AUDIT.EXPORT,
  PERMISSIONS.SECURITY.AUDIT,
  PERMISSIONS.ORGANIZATION.VIEW,
  PERMISSIONS.ORGANIZATION.MANAGE,
  PERMISSIONS.NOTIFICATION.VIEW,
  PERMISSIONS.NOTIFICATION.MANAGE,
] as const satisfies ReadonlyArray<PermissionId>
