export type InviteStatus = 'pending' | 'expired' | 'accepted'

export interface InviteRecord {
  id: string
  email: string
  invited_by?: string
  team_id?: string
  team_name?: string
  created_at: string
  expires_at: string
  accepted_at?: string | null
  status: InviteStatus
}

export interface InviteCreatePayload {
  email: string
  team_id?: string
}

export interface InviteCreateResponse {
  invite: InviteRecord
  token: string
  link?: string
}

export interface InviteRedeemResponse {
  user: {
    id: string
    username: string
    email: string
    first_name?: string
    last_name?: string
    is_active: boolean
    provider?: string
  }
  message: string
  created_user: boolean
}
