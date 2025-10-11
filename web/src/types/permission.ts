import type { PermissionId } from '@/constants/permissions'

export type PermissionIdentifier = PermissionId | (string & {})

export interface PermissionDefinition {
  id: PermissionIdentifier
  module: string
  description?: string
  depends_on: PermissionIdentifier[]
  implies: PermissionIdentifier[]
  display_name?: string
  category?: string
  default_scope?: string
  metadata?: Record<string, unknown>
}

export type PermissionRegistry = Record<string, PermissionDefinition>

export interface RoleRecord {
  id: string
  name: string
  description?: string
  is_system: boolean
  permissions?: PermissionDefinition[]
  created_at?: string
  updated_at?: string
}

export interface RoleCreatePayload {
  name: string
  description?: string
  is_system?: boolean
}

export interface RoleUpdatePayload {
  name?: string
  description?: string
}

export interface SetRolePermissionsPayload {
  permissions: PermissionIdentifier[]
}
