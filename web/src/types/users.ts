import type { ApiMeta } from '@/types/api'

export type UserStatus = 'active' | 'inactive'

export interface UserRoleSummary {
  id: string
  name: string
  description?: string
  is_system?: boolean
}

export interface UserTeamSummary {
  id: string
  name: string
}

export interface UserRecord {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_root: boolean
  is_active: boolean
  roles?: UserRoleSummary[]
  teams?: UserTeamSummary[]
  last_login_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface UserListResult {
  data: UserRecord[]
  meta?: ApiMeta
}

export interface UserListParams {
  page?: number
  per_page?: number
  search?: string
  status?: UserStatus | 'all'
}

export interface UserCreatePayload {
  username: string
  email: string
  password: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_root?: boolean
  is_active?: boolean
}

export interface UserUpdatePayload {
  username?: string
  email?: string
  first_name?: string
  last_name?: string
  avatar?: string
}

export interface BulkUserPayload {
  user_ids: string[]
}
