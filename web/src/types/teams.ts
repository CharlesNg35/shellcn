import type { ApiMeta } from '@/types/api'

export interface TeamRecord {
  id: string
  name: string
  description?: string
  created_at?: string
  updated_at?: string
  members?: TeamMember[]
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
  roles?: Array<{
    id: string
    name: string
    description?: string
  }>
}

export interface TeamMemberAssignmentPayload {
  user_id: string
}
