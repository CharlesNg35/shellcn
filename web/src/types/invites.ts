export type InviteStatus = 'pending' | 'expired' | 'accepted'

export interface InviteRecord {
  id: string
  email: string
  invited_by?: string
  created_at: string
  expires_at: string
  accepted_at?: string | null
  status: InviteStatus
}

export interface InviteCreateResponse {
  invite: InviteRecord
  token: string
  link?: string
}
