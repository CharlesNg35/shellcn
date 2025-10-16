import type { ApiResponse } from '@/types/api'
import type { SSHProtocolSettings } from '@/types/protocol-settings'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const SSH_SETTINGS_ENDPOINT = '/settings/protocols/ssh'

interface SSHProtocolSettingsResponse {
  session?: {
    concurrent_limit?: number
    idle_timeout_minutes?: number
    enable_sftp?: boolean
  }
  terminal?: {
    theme_mode?: string
    font_family?: string
    font_size?: number
    scrollback_limit?: number
    enable_webgl?: boolean
  }
  recording?: {
    mode?: string
    storage?: string
    retention_days?: number
    require_consent?: boolean
  }
  collaboration?: {
    allow_sharing?: boolean
    restrict_write_to_admins?: boolean
  }
}

function normaliseSessionSettings(payload?: SSHProtocolSettingsResponse['session']) {
  return {
    concurrent_limit:
      typeof payload?.concurrent_limit === 'number' && payload.concurrent_limit >= 0
        ? payload.concurrent_limit
        : 0,
    idle_timeout_minutes:
      typeof payload?.idle_timeout_minutes === 'number' && payload.idle_timeout_minutes >= 0
        ? payload.idle_timeout_minutes
        : 0,
    enable_sftp: payload?.enable_sftp !== false,
  }
}

function normaliseTerminalSettings(payload?: SSHProtocolSettingsResponse['terminal']) {
  return {
    theme_mode: (payload?.theme_mode ?? 'auto') as SSHProtocolSettings['terminal']['theme_mode'],
    font_family: payload?.font_family?.trim() || 'monospace',
    font_size:
      typeof payload?.font_size === 'number' && payload.font_size >= 8 ? payload.font_size : 14,
    scrollback_limit:
      typeof payload?.scrollback_limit === 'number' && payload.scrollback_limit >= 200
        ? payload.scrollback_limit
        : 1000,
    enable_webgl: payload?.enable_webgl !== false,
  }
}

function normaliseRecordingSettings(payload?: SSHProtocolSettingsResponse['recording']) {
  return {
    mode: (payload?.mode ?? 'optional') as SSHProtocolSettings['recording']['mode'],
    storage: (payload?.storage ?? 'filesystem') as SSHProtocolSettings['recording']['storage'],
    retention_days: typeof payload?.retention_days === 'number' ? payload.retention_days : 0,
    require_consent: Boolean(payload?.require_consent ?? true),
  }
}

function normaliseCollaborationSettings(payload?: SSHProtocolSettingsResponse['collaboration']) {
  return {
    allow_sharing: payload?.allow_sharing !== false,
    restrict_write_to_admins: Boolean(payload?.restrict_write_to_admins),
  }
}

function normaliseResponse(payload: SSHProtocolSettingsResponse): SSHProtocolSettings {
  return {
    session: normaliseSessionSettings(payload.session),
    terminal: normaliseTerminalSettings(payload.terminal),
    recording: normaliseRecordingSettings(payload.recording),
    collaboration: normaliseCollaborationSettings(payload.collaboration),
  }
}

export async function fetchSSHProtocolSettings(): Promise<SSHProtocolSettings> {
  const response =
    await apiClient.get<ApiResponse<SSHProtocolSettingsResponse>>(SSH_SETTINGS_ENDPOINT)
  const data = unwrapResponse(response)
  return normaliseResponse(data)
}

export async function updateSSHProtocolSettings(
  payload: SSHProtocolSettings
): Promise<SSHProtocolSettings> {
  const response = await apiClient.put<ApiResponse<SSHProtocolSettingsResponse>>(
    SSH_SETTINGS_ENDPOINT,
    payload
  )
  const data = unwrapResponse(response)
  return normaliseResponse(data)
}
