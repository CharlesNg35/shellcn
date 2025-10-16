export type IdentityScope = 'global' | 'team' | 'connection'

export type IdentitySharePermission = 'use' | 'view_metadata' | 'edit'

export type IdentitySharePrincipalType = 'user' | 'team'

export interface IdentityShareRecord {
  id: string
  principal_type: IdentitySharePrincipalType
  principal_id: string
  permission: IdentitySharePermission
  expires_at?: string | null
  metadata?: Record<string, unknown> | null
  granted_by: string
  created_by: string
  revoked_by?: string | null
  revoked_at?: string | null
}

export interface IdentityRecord {
  id: string
  name: string
  description?: string | null
  scope: IdentityScope
  owner_user_id: string
  team_id?: string | null
  connection_id?: string | null
  template_id?: string | null
  version: number
  metadata?: Record<string, unknown> | null
  usage_count: number
  last_used_at?: string | null
  last_rotated_at?: string | null
  created_at: string
  updated_at: string
  payload?: Record<string, unknown>
  shares?: IdentityShareRecord[]
  connection_count: number
}

export type CredentialFieldType = 'string' | 'secret' | 'file' | 'enum' | 'boolean' | 'number'

export type CredentialFieldInputMode = 'text' | 'file' | 'select' | 'password' | 'textarea' | string

export type CredentialFieldComparable = string | number | boolean

export interface CredentialFieldVisibilityRule {
  field: string
  equals?: CredentialFieldComparable | CredentialFieldComparable[]
  not_equals?: CredentialFieldComparable | CredentialFieldComparable[]
  in?: CredentialFieldComparable[]
  not_in?: CredentialFieldComparable[]
  exists?: boolean
  not_exists?: boolean
  truthy?: boolean
  falsy?: boolean
  mode?: 'all' | 'any'
}

export interface CredentialFieldMetadata {
  section?: string
  hint?: string
  visibility?: CredentialFieldVisibilityRule | CredentialFieldVisibilityRule[]
  visibility_mode?: 'all' | 'any'
  required_when?: CredentialFieldVisibilityRule | CredentialFieldVisibilityRule[]
  [key: string]: unknown
}

export interface CredentialField {
  name: string
  type: CredentialFieldType
  label?: string
  description?: string
  required?: boolean
  placeholder?: string
  default_value?: unknown
  input_modes?: CredentialFieldInputMode[]
  options?: Array<string | Record<string, unknown>>
  metadata?: CredentialFieldMetadata
  validation?: Record<string, unknown>
  key?: string
  [key: string]: unknown
}

export interface CredentialTemplateRecord {
  id: string
  driver_id: string
  version: string
  display_name: string
  description?: string | null
  fields: CredentialField[]
  compatible_protocols: string[]
  deprecated_after?: string | null
  metadata?: Record<string, unknown> | null
  hash: string
}

export interface IdentityListParams {
  scope?: IdentityScope | 'all'
  protocol_id?: string
  include_connection_scoped?: boolean
}

export interface IdentityCreatePayload {
  name: string
  description?: string
  scope: IdentityScope
  template_id?: string | null
  team_id?: string | null
  connection_id?: string | null
  metadata?: Record<string, unknown>
  payload: Record<string, unknown>
  owner_user_id?: string
}

export interface IdentityUpdatePayload {
  name?: string
  description?: string
  template_id?: string | null
  connection_id?: string | null
  metadata?: Record<string, unknown>
  payload?: Record<string, unknown>
}

export interface IdentitySharePayload {
  principal_type: IdentitySharePrincipalType
  principal_id: string
  permission: IdentitySharePermission
  expires_at?: string | null
  metadata?: Record<string, unknown>
}
