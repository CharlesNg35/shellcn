import type { ApiResponse } from '@/types/api'
import type {
  PermissionDefinition,
  PermissionRegistry,
  RoleCreatePayload,
  RoleRecord,
  RoleUpdatePayload,
  SetRolePermissionsPayload,
} from '@/types/permission'
import type { PermissionId } from '@/constants/permissions'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const PERMISSIONS_REGISTRY_ENDPOINT = '/permissions/registry'
const MY_PERMISSIONS_ENDPOINT = '/permissions/my'
const ROLES_ENDPOINT = '/permissions/roles'

interface PermissionRegistryEntryResponse {
  id?: string
  module?: string
  description?: string
  depends_on?: unknown
  implies?: unknown
}

type PermissionRegistryResponse = Record<string, PermissionRegistryEntryResponse>

interface RoleResponse {
  id: string
  name: string
  description?: string
  is_system: boolean
  permissions?: PermissionRegistryEntryResponse[] | null
  created_at?: string
  updated_at?: string
}

function normalisePermissionIds(source: unknown): string[] {
  if (Array.isArray(source)) {
    return source
      .map((value) => (typeof value === 'string' ? value.trim() : String(value).trim()))
      .filter((value) => value.length > 0)
  }

  if (typeof source === 'string' && source.trim().length > 0) {
    try {
      const parsed = JSON.parse(source) as unknown
      if (Array.isArray(parsed)) {
        return parsed
          .map((value) => (typeof value === 'string' ? value.trim() : String(value).trim()))
          .filter((value) => value.length > 0)
      }
    } catch {
      return [source.trim()]
    }
  }

  return []
}

function toPermissionId(value: unknown, fallback?: string): string | null {
  if (typeof value === 'string' && value.trim().length > 0) {
    return value.trim()
  }
  if (fallback && fallback.trim().length > 0) {
    return fallback.trim()
  }
  return null
}

function transformPermission(
  raw: PermissionRegistryEntryResponse,
  fallbackId?: string
): PermissionDefinition | null {
  const id = toPermissionId(raw.id, fallbackId)
  if (!id) {
    return null
  }

  return {
    id,
    module: toPermissionId(raw.module, 'core') ?? 'core',
    description: raw.description,
    depends_on: normalisePermissionIds(raw.depends_on),
    implies: normalisePermissionIds(raw.implies),
  }
}

function transformRegistry(payload: PermissionRegistryResponse): PermissionRegistry {
  const entries: Array<[string, PermissionDefinition]> = []

  Object.entries(payload ?? {}).forEach(([id, definition]) => {
    const transformed = transformPermission(definition, id)
    if (transformed) {
      entries.push([transformed.id, transformed])
    }
  })

  return Object.fromEntries(entries)
}

function transformRole(raw: RoleResponse): RoleRecord {
  const permissions =
    raw.permissions
      ?.map((permission) => transformPermission(permission, permission.id))
      .filter((permission): permission is PermissionDefinition => permission !== null) ?? []

  return {
    id: raw.id,
    name: raw.name,
    description: raw.description,
    is_system: Boolean(raw.is_system),
    permissions,
    created_at: raw.created_at,
    updated_at: raw.updated_at,
  }
}

export async function fetchPermissionRegistry(): Promise<PermissionRegistry> {
  const response = await apiClient.get<ApiResponse<PermissionRegistryResponse>>(
    PERMISSIONS_REGISTRY_ENDPOINT
  )
  const data = unwrapResponse(response)
  return transformRegistry(data)
}

export async function fetchMyPermissions(): Promise<PermissionId[]> {
  const response = await apiClient.get<ApiResponse<PermissionId[]>>(MY_PERMISSIONS_ENDPOINT)
  return unwrapResponse(response)
}

export async function fetchRoles(): Promise<RoleRecord[]> {
  const response = await apiClient.get<ApiResponse<RoleResponse[]>>(ROLES_ENDPOINT)
  const data = unwrapResponse(response)
  return data.map(transformRole)
}

export async function createRole(payload: RoleCreatePayload): Promise<RoleRecord> {
  const response = await apiClient.post<ApiResponse<RoleResponse>>(ROLES_ENDPOINT, payload)
  const data = unwrapResponse(response)
  return transformRole(data)
}

export async function updateRole(roleId: string, payload: RoleUpdatePayload): Promise<RoleRecord> {
  const response = await apiClient.patch<ApiResponse<RoleResponse>>(
    `${ROLES_ENDPOINT}/${roleId}`,
    payload
  )
  const data = unwrapResponse(response)
  return transformRole(data)
}

export async function deleteRole(roleId: string): Promise<boolean> {
  const response = await apiClient.delete<ApiResponse<{ deleted: boolean }>>(
    `${ROLES_ENDPOINT}/${roleId}`
  )
  const data = unwrapResponse(response)
  return Boolean((data as { deleted?: boolean })?.deleted)
}

export async function setRolePermissions(
  roleId: string,
  payload: SetRolePermissionsPayload
): Promise<boolean> {
  const response = await apiClient.post<ApiResponse<{ updated: boolean }>>(
    `${ROLES_ENDPOINT}/${roleId}/permissions`,
    payload
  )
  const data = unwrapResponse(response)
  return Boolean((data as { updated?: boolean })?.updated)
}

export const permissionsApi = {
  getRegistry: fetchPermissionRegistry,
  getMyPermissions: fetchMyPermissions,
  listRoles: fetchRoles,
  createRole,
  updateRole,
  deleteRole,
  setRolePermissions,
}
