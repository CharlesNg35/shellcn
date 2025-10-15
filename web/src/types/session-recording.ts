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
