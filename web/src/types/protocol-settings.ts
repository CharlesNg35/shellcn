export type RecordingMode = 'disabled' | 'optional' | 'forced'
export type RecordingStorage = 'filesystem' | 's3'

export interface SSHRecordingSettings {
  mode: RecordingMode
  storage: RecordingStorage
  retention_days: number
  require_consent: boolean
}

export interface SSHProtocolSettings {
  recording: SSHRecordingSettings
}
