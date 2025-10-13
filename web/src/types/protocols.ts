export interface ProtocolCapabilities {
  terminal: boolean
  desktop: boolean
  file_transfer: boolean
  clipboard: boolean
  session_recording: boolean
  metrics: boolean
  reconnect: boolean
  extras: Record<string, boolean>
}

export interface ProtocolPermission {
  id: string
  display_name?: string
  description?: string
  category?: string
  default_scope?: string
  module?: string
  depends_on?: string[]
  implies?: string[]
  metadata?: Record<string, unknown>
}

export interface Protocol {
  id: string
  name: string
  module: string
  description?: string
  category: string
  icon?: string
  defaultPort?: number
  sortOrder?: number
  features: string[]
  capabilities: ProtocolCapabilities
  driverEnabled: boolean
  configEnabled: boolean
  available: boolean
  permissions: ProtocolPermission[]
}

export interface ProtocolListResult {
  data: Protocol[]
  count: number
}
