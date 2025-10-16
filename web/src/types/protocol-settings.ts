export type RecordingMode = 'disabled' | 'optional' | 'forced'
export type RecordingStorage = 'filesystem' | 's3'
export type SSHThemeMode = 'auto' | 'force_dark' | 'force_light'

export interface SSHSessionSettings {
  concurrent_limit: number
  idle_timeout_minutes: number
  enable_sftp: boolean
}

export interface SSHTerminalSettings {
  theme_mode: SSHThemeMode
  font_family: string
  font_size: number
  scrollback_limit: number
  enable_webgl: boolean
}

export interface SSHRecordingSettings {
  mode: RecordingMode
  storage: RecordingStorage
  retention_days: number
  require_consent: boolean
}

export interface SSHCollaborationSettings {
  allow_sharing: boolean
  restrict_write_to_admins: boolean
}

export interface SSHProtocolSettings {
  session: SSHSessionSettings
  terminal: SSHTerminalSettings
  recording: SSHRecordingSettings
  collaboration: SSHCollaborationSettings
}
