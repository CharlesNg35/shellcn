import type { ApiResponse } from '@/types/api'
import type { PermissionRecord } from '@/types/permission'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const PERMISSIONS_ENDPOINT = '/permissions'
const MY_PERMISSIONS_ENDPOINT = '/permissions/my'
const ROLES_ENDPOINT = '/permissions/roles'

export async function fetchPermissions(): Promise<PermissionRecord[]> {
  const response = await apiClient.get<ApiResponse<PermissionRecord[]>>(PERMISSIONS_ENDPOINT)
  return unwrapResponse(response)
}

export async function fetchMyPermissions(): Promise<string[]> {
  const response = await apiClient.get<ApiResponse<string[]>>(MY_PERMISSIONS_ENDPOINT)
  return unwrapResponse(response)
}

export async function fetchRolePermissions(roleId: string): Promise<PermissionRecord[]> {
  const response = await apiClient.get<ApiResponse<PermissionRecord[]>>(
    `${ROLES_ENDPOINT}/${roleId}`
  )
  return unwrapResponse(response)
}

export async function assignPermissionToRole(roleId: string, permissionId: string): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(`${ROLES_ENDPOINT}/${roleId}`, {
    permission_id: permissionId,
  })
  unwrapResponse(response)
}

export async function removePermissionFromRole(
  roleId: string,
  permissionId: string
): Promise<void> {
  const response = await apiClient.delete<ApiResponse<unknown>>(
    `${ROLES_ENDPOINT}/${roleId}/${permissionId}`
  )
  unwrapResponse(response)
}

export const permissionsApi = {
  getAll: fetchPermissions,
  getMyPermissions: fetchMyPermissions,
  getRolePermissions: fetchRolePermissions,
  assignToRole: assignPermissionToRole,
  removeFromRole: removePermissionFromRole,
}
