import type { ApiMeta } from '@/types/api'

export type UserStatus = 'active' | 'inactive'

export interface UserRoleSummary {
  id: string
  name: string
  description?: string
}

export interface UserTeamSummary {
  id: string
  name: string
}

export interface UserOrganizationSummary {
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
  organization_id?: string | null
  organization?: UserOrganizationSummary | null
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
  organization_id?: string
}

export interface UserCreatePayload {
  username: string
  email: string
  password: string
  first_name?: string
  last_name?: string
  avatar?: string
  organization_id?: string | null
  is_root?: boolean
  is_active?: boolean
}

export interface UserUpdatePayload {
  username?: string
  email?: string
  first_name?: string
  last_name?: string
  avatar?: string
  organization_id?: string | null
}

export interface BulkUserPayload {
  user_ids: string[]
}
