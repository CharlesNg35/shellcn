import type { ApiResponse } from '@/types/api'
import type { SessionRecordingStatus } from '@/types/session-recording'
import { apiClient } from './client'
import { unwrapResponse } from './http'

interface SessionRecordingStatusResponse {
  session_id: string
  active?: boolean
  started_at?: string | null
  last_event_at?: string | null
  bytes_recorded?: number
  size_bytes?: number
  storage_path?: string | null
  record_id?: string | null
  duration_seconds?: number
  storage_kind?: string | null
  checksum?: string | null
  recording_mode?: string | null
  created_at?: string | null
  retention_until?: string | null
  record?: {
    record_id?: string
    session_id?: string
    storage_path?: string
    storage_kind?: string
    size_bytes?: number
    duration_seconds?: number
    checksum?: string | null
    created_at?: string | null
    retention_until?: string | null
  } | null
}

function normaliseStatus(payload: SessionRecordingStatusResponse): SessionRecordingStatus {
  return {
    session_id: payload.session_id,
    active: Boolean(payload.active),
    started_at: payload.started_at ?? undefined,
    last_event_at: payload.last_event_at ?? undefined,
    bytes_recorded:
      typeof payload.bytes_recorded === 'number'
        ? payload.bytes_recorded
        : typeof payload.size_bytes === 'number'
          ? payload.size_bytes
          : 0,
    recording_mode: payload.recording_mode ?? undefined,
    record: payload.record
      ? {
          record_id: payload.record.record_id ?? '',
          session_id: payload.record.session_id ?? payload.session_id,
          storage_path: payload.record.storage_path ?? '',
          storage_kind: payload.record.storage_kind ?? 'filesystem',
          size_bytes: payload.record.size_bytes ?? 0,
          duration_seconds: payload.record.duration_seconds ?? 0,
          checksum: payload.record.checksum ?? undefined,
          created_at: payload.record.created_at ?? undefined,
          retention_until: payload.record.retention_until ?? undefined,
        }
      : payload.record_id
        ? {
            record_id: payload.record_id,
            session_id: payload.session_id,
            storage_path: payload.storage_path ?? '',
            storage_kind: payload.storage_kind ?? 'filesystem',
            size_bytes: payload.bytes_recorded ?? payload.size_bytes ?? 0,
            duration_seconds: payload.duration_seconds ?? 0,
            checksum: payload.checksum ?? undefined,
            created_at: payload.created_at ?? undefined,
            retention_until: payload.retention_until ?? undefined,
          }
        : null,
  }
}

export async function fetchSessionRecordingStatus(
  sessionId: string
): Promise<SessionRecordingStatus> {
  const response = await apiClient.get<ApiResponse<SessionRecordingStatusResponse>>(
    `/active-sessions/${sessionId}/recording/status`
  )
  const data = unwrapResponse(response)
  return normaliseStatus(data)
}

export async function stopSessionRecording(sessionId: string): Promise<SessionRecordingStatus> {
  const response = await apiClient.post<ApiResponse<SessionRecordingStatusResponse>>(
    `/active-sessions/${sessionId}/recording/stop`
  )
  const data = unwrapResponse(response)
  return normaliseStatus(data)
}

export async function downloadSessionRecording(recordId: string): Promise<Blob> {
  const response = await apiClient.get(`/session-records/${recordId}/download`, {
    responseType: 'blob',
  })
  return response.data as Blob
}
