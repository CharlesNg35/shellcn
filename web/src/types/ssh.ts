export interface SshTerminalEvent {
  stream: string
  event: string
  sessionId: string
  connectionId?: string
  channel?: string
  encoding?: string
  text?: string
  raw?: Uint8Array
  message?: string
  original?: Record<string, unknown>
}

export interface SshLatencySample {
  sessionId: string
  latencyMs: number
  recordedAt: Date
}
