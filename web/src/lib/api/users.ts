import type { ApiMeta, ApiResponse } from '@/types/api'
import { isApiSuccess } from '@/types/api'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import type {
  BulkUserPayload,
  UserCreatePayload,
  UserListParams,
  UserListResult,
  UserRecord,
  UserUpdatePayload,
} from '@/types/users'

const USERS_ENDPOINT = '/users'
const BULK_ACTIVATE_ENDPOINT = '/users/bulk/activate'
const BULK_DEACTIVATE_ENDPOINT = '/users/bulk/deactivate'
const BULK_DELETE_ENDPOINT = '/users/bulk'

interface UserResponse {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_root: boolean
  is_active: boolean
  auth_provider?: string
  auth_subject?: string
  roles?: Array<{
    id: string
    name: string
    description?: string
    is_system?: boolean
  }>
  teams?: Array<{
    id: string
    name: string
  }>
  last_login_at?: string | null
  created_at?: string
  updated_at?: string
}

interface BulkOperationResponse {
  updated?: number
  deleted?: number
  failed?: Record<string, string>
  is_active?: boolean
}

function transformUser(raw: UserResponse): UserRecord {
  return {
    id: raw.id,
    username: raw.username,
    email: raw.email,
    first_name: raw.first_name,
    last_name: raw.last_name,
    avatar: raw.avatar,
    is_root: raw.is_root,
    is_active: raw.is_active,
    auth_provider: raw.auth_provider,
    auth_subject: raw.auth_subject,
    roles:
      raw.roles?.map((role) => ({
        id: role.id,
        name: role.name,
        description: role.description,
        is_system: role.is_system,
      })) ?? [],
    teams: raw.teams ?? [],
    last_login_at: raw.last_login_at,
    created_at: raw.created_at,
    updated_at: raw.updated_at,
  }
}

export async function fetchUsers(params: UserListParams = {}): Promise<UserListResult> {
  const queryParams: Record<string, unknown> = {}
  if (params.page) {
    queryParams.page = params.page
  }
  if (params.per_page) {
    queryParams.per_page = params.per_page
  }
  if (params.search) {
    queryParams.search = params.search
  }
  if (params.status && params.status !== 'all') {
    queryParams.status = params.status
  }

  const response = await apiClient.get<ApiResponse<UserResponse[]>>(USERS_ENDPOINT, {
    params: queryParams,
  })

  const payload = response.data
  const data = unwrapResponse(response)
  const meta: ApiMeta | undefined = isApiSuccess(payload) ? payload.meta : undefined

  return {
    data: data.map(transformUser),
    meta,
  }
}

export async function fetchUserById(userId: string): Promise<UserRecord> {
  const response = await apiClient.get<ApiResponse<UserResponse>>(`${USERS_ENDPOINT}/${userId}`)
  const data = unwrapResponse(response)
  return transformUser(data)
}

export async function createUser(payload: UserCreatePayload): Promise<UserRecord> {
  const response = await apiClient.post<ApiResponse<UserResponse>>(USERS_ENDPOINT, payload)
  const data = unwrapResponse(response)
  return transformUser(data)
}

export async function updateUser(userId: string, payload: UserUpdatePayload): Promise<UserRecord> {
  const response = await apiClient.patch<ApiResponse<UserResponse>>(
    `${USERS_ENDPOINT}/${userId}`,
    payload
  )
  const data = unwrapResponse(response)
  return transformUser(data)
}

export async function activateUser(userId: string): Promise<UserRecord> {
  const response = await apiClient.post<ApiResponse<UserResponse>>(
    `${USERS_ENDPOINT}/${userId}/activate`
  )
  return transformUser(unwrapResponse(response))
}

export async function deactivateUser(userId: string): Promise<UserRecord> {
  const response = await apiClient.post<ApiResponse<UserResponse>>(
    `${USERS_ENDPOINT}/${userId}/deactivate`
  )
  return transformUser(unwrapResponse(response))
}

export async function changeUserPassword(userId: string, password: string): Promise<void> {
  await apiClient.post<ApiResponse<Record<string, unknown>>>(
    `${USERS_ENDPOINT}/${userId}/password`,
    {
      password,
    }
  )
}

export async function setUserRoles(userId: string, roleIds: string[]): Promise<UserRecord> {
  const response = await apiClient.put<ApiResponse<UserResponse>>(
    `${USERS_ENDPOINT}/${userId}/roles`,
    {
      role_ids: roleIds,
    }
  )
  const data = unwrapResponse(response)
  return transformUser(data)
}

async function bulkOperation(
  endpoint: string,
  payload: BulkUserPayload & { active?: boolean }
): Promise<BulkOperationResponse> {
  const response = await apiClient.post<ApiResponse<BulkOperationResponse>>(endpoint, payload)
  return unwrapResponse(response)
}

export async function bulkActivateUsers(userIds: string[]): Promise<BulkOperationResponse> {
  return bulkOperation(BULK_ACTIVATE_ENDPOINT, { user_ids: userIds, active: true })
}

export async function bulkDeactivateUsers(userIds: string[]): Promise<BulkOperationResponse> {
  return bulkOperation(BULK_DEACTIVATE_ENDPOINT, { user_ids: userIds, active: false })
}

export async function bulkDeleteUsers(userIds: string[]): Promise<BulkOperationResponse> {
  const response = await apiClient.delete<ApiResponse<BulkOperationResponse>>(
    BULK_DELETE_ENDPOINT,
    {
      data: { user_ids: userIds },
    }
  )
  return unwrapResponse(response)
}

export const usersApi = {
  list: fetchUsers,
  get: fetchUserById,
  create: createUser,
  update: updateUser,
  activate: activateUser,
  deactivate: deactivateUser,
  changePassword: changeUserPassword,
  setRoles: setUserRoles,
  bulkActivate: bulkActivateUsers,
  bulkDeactivate: bulkDeactivateUsers,
  bulkDelete: bulkDeleteUsers,
}
