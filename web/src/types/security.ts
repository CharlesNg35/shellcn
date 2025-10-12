export type SecurityCheckStatus = 'pass' | 'warn' | 'fail'

export interface SecurityAuditCheck {
  id: string
  status: SecurityCheckStatus
  message: string
  remediation?: string
  details?: unknown
}

export interface SecurityAuditSummary {
  pass?: number
  warn?: number
  fail?: number
  [key: string]: number | undefined
}

export interface SecurityAuditResult {
  checked_at: string
  checks: SecurityAuditCheck[]
  summary: SecurityAuditSummary
}
