export interface PermissionRecord {
  id: string
  name: string
  description?: string
  category?: string
  module?: string
  depends_on?: string[]
}

export interface PermissionGroup {
  id: string
  name: string
  permissions: PermissionRecord[]
}

export interface RolePermissionSummary {
  role_id: string
  permission_id: string
}
