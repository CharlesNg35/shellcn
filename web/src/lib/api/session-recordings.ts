import type { ApiMeta, ApiResponse } from '@/types/api'
import { isApiSuccess } from '@/types/api'
import type {
  SessionRecordingScope,
  SessionRecordingStatus,
  SessionRecordingSummary,
} from '@/types/session-recording'
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

interface SessionRecordingSummaryResponse {
  record_id: string
  session_id: string
  connection_id: string
  connection_name?: string | null
  protocol_id: string
  owner_user_id: string
  owner_user_name?: string | null
  team_id?: string | null
  created_by_user_id: string
  created_by_user_name?: string | null
  storage_kind: string
  storage_path: string
  size_bytes: number
  duration_seconds: number
  checksum?: string | null
  created_at: string
  retention_until?: string | null
}

export interface FetchSessionRecordingsParams {
  scope?: SessionRecordingScope
  team_id?: string
  protocol_id?: string
  connection_id?: string
  owner_user_id?: string
  created_by_user_id?: string
  page?: number
  per_page?: number
  sort?: 'recent' | 'oldest' | 'size_desc' | 'size_asc'
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

function transformSummary(payload: SessionRecordingSummaryResponse): SessionRecordingSummary {
  return {
    record_id: payload.record_id,
    session_id: payload.session_id,
    connection_id: payload.connection_id,
    connection_name: payload.connection_name ?? undefined,
    protocol_id: payload.protocol_id,
    owner_user_id: payload.owner_user_id,
    owner_user_name: payload.owner_user_name ?? undefined,
    team_id: payload.team_id ?? undefined,
    created_by_user_id: payload.created_by_user_id,
    created_by_user_name: payload.created_by_user_name ?? undefined,
    storage_kind: payload.storage_kind,
    storage_path: payload.storage_path,
    size_bytes: payload.size_bytes,
    duration_seconds: payload.duration_seconds,
    checksum: payload.checksum ?? undefined,
    created_at: payload.created_at,
    retention_until: payload.retention_until ?? undefined,
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

export async function fetchSessionRecordings(params?: FetchSessionRecordingsParams): Promise<{
  data: SessionRecordingSummary[]
  meta?: ApiMeta
}> {
  const response = await apiClient.get<ApiResponse<SessionRecordingSummaryResponse[]>>(
    '/session-records',
    {
      params,
    }
  )
  const payload = response.data
  const data = unwrapResponse(response)
  const summaries = Array.isArray(data) ? data.map(transformSummary) : []
  const meta = isApiSuccess(payload) ? payload.meta : undefined
  return {
    data: summaries,
    meta,
  }
}

export async function deleteSessionRecording(recordId: string): Promise<void> {
  await apiClient.delete(`/session-records/${recordId}`)
}

export async function downloadSessionRecording(recordId: string): Promise<Blob> {
  const response = await apiClient.get(`/session-records/${recordId}/download`, {
    responseType: 'blob',
  })
  return response.data as Blob
}
