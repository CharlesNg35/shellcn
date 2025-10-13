export const PERMISSIONS = {
  USER: {
    VIEW: 'user.view',
    CREATE: 'user.create',
    EDIT: 'user.edit',
    DELETE: 'user.delete',
    INVITE: 'user.invite',
  },
  TEAM: {
    VIEW: 'team.view',
    VIEW_ALL: 'team.view_all',
    CREATE: 'team.create',
    UPDATE: 'team.update',
    DELETE: 'team.delete',
    MEMBER: {
      ADD: 'team.member.add',
      REMOVE: 'team.member.remove',
      MANAGE: 'team.member.manage',
    },
    MANAGE: 'team.manage',
  },
  CONNECTION: {
    VIEW: 'connection.view',
    VIEW_ALL: 'connection.view_all',
    CREATE: 'connection.create',
    UPDATE: 'connection.update',
    DELETE: 'connection.delete',
    LAUNCH: 'connection.launch',
    SHARE: 'connection.share',
    MANAGE: 'connection.manage',
  },
  CONNECTION_FOLDER: {
    VIEW: 'connection.folder.view',
    CREATE: 'connection.folder.create',
    UPDATE: 'connection.folder.update',
    DELETE: 'connection.folder.delete',
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
  PERMISSIONS.USER.INVITE,
  PERMISSIONS.TEAM.VIEW,
  PERMISSIONS.TEAM.VIEW_ALL,
  PERMISSIONS.TEAM.CREATE,
  PERMISSIONS.TEAM.UPDATE,
  PERMISSIONS.TEAM.DELETE,
  PERMISSIONS.TEAM.MEMBER.ADD,
  PERMISSIONS.TEAM.MEMBER.REMOVE,
  PERMISSIONS.TEAM.MEMBER.MANAGE,
  PERMISSIONS.TEAM.MANAGE,
  PERMISSIONS.CONNECTION.VIEW,
  PERMISSIONS.CONNECTION.VIEW_ALL,
  PERMISSIONS.CONNECTION.CREATE,
  PERMISSIONS.CONNECTION.UPDATE,
  PERMISSIONS.CONNECTION.DELETE,
  PERMISSIONS.CONNECTION.LAUNCH,
  PERMISSIONS.CONNECTION.SHARE,
  PERMISSIONS.CONNECTION.MANAGE,
  PERMISSIONS.CONNECTION_FOLDER.VIEW,
  PERMISSIONS.CONNECTION_FOLDER.CREATE,
  PERMISSIONS.CONNECTION_FOLDER.UPDATE,
  PERMISSIONS.CONNECTION_FOLDER.DELETE,
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
  PERMISSIONS.NOTIFICATION.VIEW,
  PERMISSIONS.NOTIFICATION.MANAGE,
] as const satisfies ReadonlyArray<PermissionId>
