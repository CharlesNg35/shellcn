import type { ApiResponse } from '@/types/api'
import type { SSHProtocolSettings } from '@/types/protocol-settings'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const SSH_SETTINGS_ENDPOINT = '/settings/protocols/ssh'

interface SSHProtocolSettingsResponse {
  recording?: {
    mode?: string
    storage?: string
    retention_days?: number
    require_consent?: boolean
  }
}

function normaliseRecordingSettings(payload?: SSHProtocolSettingsResponse['recording']) {
  return {
    mode: (payload?.mode ?? 'optional') as SSHProtocolSettings['recording']['mode'],
    storage: (payload?.storage ?? 'filesystem') as SSHProtocolSettings['recording']['storage'],
    retention_days: typeof payload?.retention_days === 'number' ? payload.retention_days : 0,
    require_consent: Boolean(payload?.require_consent),
  }
}

export async function fetchSSHProtocolSettings(): Promise<SSHProtocolSettings> {
  const response =
    await apiClient.get<ApiResponse<SSHProtocolSettingsResponse>>(SSH_SETTINGS_ENDPOINT)
  const data = unwrapResponse(response)
  return {
    recording: normaliseRecordingSettings(data.recording),
  }
}

export async function updateSSHProtocolSettings(
  payload: SSHProtocolSettings
): Promise<SSHProtocolSettings> {
  const response = await apiClient.put<ApiResponse<SSHProtocolSettingsResponse>>(
    SSH_SETTINGS_ENDPOINT,
    payload
  )
  const data = unwrapResponse(response)
  return {
    recording: normaliseRecordingSettings(data.recording),
  }
}
