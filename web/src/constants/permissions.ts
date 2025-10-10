export const PERMISSIONS = {
  USER: {
    VIEW: 'user.view',
    CREATE: 'user.create',
    EDIT: 'user.edit',
    DELETE: 'user.delete',
    MANAGE: 'user.manage',
  },
  TEAM: {
    VIEW: 'team.view',
    MANAGE: 'team.manage',
  },
  CONNECTION: {
    VIEW: 'connection.view',
    MANAGE: 'connection.manage',
  },
  CONNECTION_FOLDER: {
    VIEW: 'connection.folder.view',
    MANAGE: 'connection.folder.manage',
  },
  PERMISSION: {
    VIEW: 'permission.view',
    MANAGE: 'permission.manage',
  },
  AUDIT: {
    VIEW: 'audit.view',
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
  PERMISSIONS.USER.MANAGE,
  PERMISSIONS.TEAM.VIEW,
  PERMISSIONS.TEAM.MANAGE,
  PERMISSIONS.CONNECTION.VIEW,
  PERMISSIONS.CONNECTION.MANAGE,
  PERMISSIONS.CONNECTION_FOLDER.VIEW,
  PERMISSIONS.CONNECTION_FOLDER.MANAGE,
  PERMISSIONS.PERMISSION.VIEW,
  PERMISSIONS.PERMISSION.MANAGE,
  PERMISSIONS.AUDIT.VIEW,
  PERMISSIONS.ORGANIZATION.VIEW,
  PERMISSIONS.ORGANIZATION.MANAGE,
  PERMISSIONS.NOTIFICATION.VIEW,
  PERMISSIONS.NOTIFICATION.MANAGE,
] as const satisfies ReadonlyArray<PermissionId>
