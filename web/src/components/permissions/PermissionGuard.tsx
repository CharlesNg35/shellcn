import type { ReactNode } from 'react'
import { usePermissions } from '@/hooks/usePermissions'
import type { PermissionId } from '@/constants/permissions'

interface PermissionGuardProps {
  permission?: PermissionId
  anyOf?: ReadonlyArray<PermissionId>
  allOf?: ReadonlyArray<PermissionId>
  fallback?: ReactNode
  loadingFallback?: ReactNode
  children: ReactNode
}

export function PermissionGuard({
  permission,
  anyOf,
  allOf,
  fallback = null,
  loadingFallback = null,
  children,
}: PermissionGuardProps) {
  const { hasPermission, hasAnyPermission, hasAllPermissions, isLoading } = usePermissions()

  if (isLoading) {
    return loadingFallback
  }

  const checks: boolean[] = []

  if (permission) {
    checks.push(hasPermission(permission))
  }

  if (allOf?.length) {
    checks.push(hasAllPermissions(allOf))
  }

  if (anyOf?.length) {
    checks.push(hasAnyPermission(anyOf))
  }

  const canRender = checks.length ? checks.every(Boolean) : true

  if (!canRender) {
    return <>{fallback}</>
  }

  return <>{children}</>
}
