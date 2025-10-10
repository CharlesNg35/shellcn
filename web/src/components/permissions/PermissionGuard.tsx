import type { ReactNode } from 'react'
import { usePermissions } from '@/hooks/usePermissions'
import type { PermissionIdentifier } from '@/types/permission'

interface PermissionGuardProps {
  permission?: PermissionIdentifier | null
  anyOf?: ReadonlyArray<PermissionIdentifier>
  allOf?: ReadonlyArray<PermissionIdentifier>
  not?: ReadonlyArray<PermissionIdentifier>
  fallback?: ReactNode
  loadingFallback?: ReactNode
  children: ReactNode
}

export function PermissionGuard({
  permission,
  anyOf,
  allOf,
  not,
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

  if (not?.length) {
    checks.push(!hasAnyPermission(not))
  }

  const canRender = checks.length ? checks.every(Boolean) : true

  if (!canRender) {
    return <>{fallback}</>
  }

  return <>{children}</>
}
