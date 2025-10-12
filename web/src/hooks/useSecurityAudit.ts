import { useQuery } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { securityApi } from '@/lib/api/security'
import type { SecurityAuditResult } from '@/types/security'

export const SECURITY_AUDIT_QUERY_KEY = ['security', 'audit'] as const

export function useSecurityAudit() {
  return useQuery<SecurityAuditResult, ApiError>({
    queryKey: SECURITY_AUDIT_QUERY_KEY,
    queryFn: securityApi.fetchAudit,
    staleTime: 60_000,
  })
}
