import type { ApiMeta } from '@/types/api'
import type { UserRoleSummary } from '@/types/users'

export interface TeamRecord {
  id: string
  name: string
  description?: string
  created_at?: string
  updated_at?: string
  source?: string
  external_id?: string
  members?: TeamMember[]
  roles?: UserRoleSummary[]
}

export interface TeamListResult {
  data: TeamRecord[]
  meta?: ApiMeta
}

export interface TeamCreatePayload {
  name: string
  description?: string
}

export interface TeamUpdatePayload {
  name?: string
  description?: string
}

export interface TeamMember {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_active: boolean
  is_root?: boolean
  last_login_at?: string | null
  roles?: UserRoleSummary[]
}

export interface TeamMemberAssignmentPayload {
  user_id: string
}

export interface TeamResourceGrant {
  resource_id: string
  resource_type: string
  permission_id: string
  expires_at?: string | null
}

export interface TeamCapabilities {
  team_id: string
  permission_ids: string[]
  resource_grants: TeamResourceGrant[]
}
