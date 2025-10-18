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
  connectionTemplateVersion?: string
  identityRequired: boolean
  permissions: ProtocolPermission[]
}

export interface ProtocolListResult {
  data: Protocol[]
  count: number
}

export type ConnectionTemplateFieldType =
  | 'string'
  | 'multiline'
  | 'number'
  | 'boolean'
  | 'select'
  | 'target_host'
  | 'target_port'
  | 'json'

export interface ConnectionTemplateBinding {
  target: 'settings' | 'metadata' | 'target'
  path?: string
  index?: number
  property?: string
}

export interface ConnectionTemplateOption {
  value: string
  label: string
}

export interface ConnectionTemplateFieldDependency {
  field: string
  equals?: unknown
}

export interface ConnectionTemplateField {
  key: string
  label: string
  type: ConnectionTemplateFieldType
  required: boolean
  default?: unknown
  placeholder?: string
  helpText?: string
  options?: ConnectionTemplateOption[]
  binding?: ConnectionTemplateBinding
  validation?: Record<string, unknown>
  dependencies?: ConnectionTemplateFieldDependency[]
  metadata?: Record<string, unknown>
}

export interface ConnectionTemplateSection {
  id: string
  label: string
  description?: string
  fields: ConnectionTemplateField[]
  metadata?: Record<string, unknown>
}

export interface ConnectionTemplate {
  driverId: string
  version: string
  displayName: string
  description?: string
  protocols?: string[]
  sections: ConnectionTemplateSection[]
  metadata?: Record<string, unknown>
}
