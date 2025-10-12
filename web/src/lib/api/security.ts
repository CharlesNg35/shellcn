import type { ApiResponse } from '@/types/api'
import type { SecurityAuditResult } from '@/types/security'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const SECURITY_AUDIT_ENDPOINT = '/security/audit'

export async function fetchSecurityAudit(): Promise<SecurityAuditResult> {
  const response = await apiClient.get<ApiResponse<SecurityAuditResult>>(SECURITY_AUDIT_ENDPOINT)
  return unwrapResponse(response)
}

export const securityApi = {
  fetchAudit: fetchSecurityAudit,
}
