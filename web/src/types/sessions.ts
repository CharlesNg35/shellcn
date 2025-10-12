export type SessionStatus = 'active' | 'revoked' | 'expired'

export interface SessionPayload {
  id: string
  user_id: string
  ip_address?: string | null
  user_agent?: string | null
  device_name?: string | null
  expires_at: string
  last_used_at: string
  created_at: string
  updated_at?: string
  revoked_at?: string | null
}
