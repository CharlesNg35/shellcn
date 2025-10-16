export interface SessionRecordingRecord {
  record_id: string
  session_id: string
  storage_path: string
  storage_kind: string
  size_bytes: number
  duration_seconds: number
  checksum?: string
  created_at?: string
  retention_until?: string
}

export interface SessionRecordingStatus {
  session_id: string
  active: boolean
  started_at?: string
  last_event_at?: string
  bytes_recorded: number
  recording_mode?: string
  record: SessionRecordingRecord | null
}

export type SessionRecordingScope = 'personal' | 'team' | 'all'

export interface SessionRecordingSummary {
  record_id: string
  session_id: string
  connection_id: string
  connection_name?: string
  protocol_id: string
  owner_user_id: string
  owner_user_name?: string
  team_id?: string | null
  created_by_user_id: string
  created_by_user_name?: string
  storage_kind: string
  storage_path: string
  size_bytes: number
  duration_seconds: number
  checksum?: string
  created_at: string
  retention_until?: string | null
}
