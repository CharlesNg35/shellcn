import { useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { permissionsApi } from '@/lib/api/permissions'
import { useCurrentUser } from './useCurrentUser'
import type { AuthUser } from '@/types/auth'
import type { PermissionId } from '@/constants/permissions'

export const MY_PERMISSIONS_QUERY_KEY = ['permissions', 'my'] as const

export interface UsePermissionsResult {
  permissions: PermissionId[]
  hasPermission: (permissionId: PermissionId | null | undefined) => boolean
  hasAnyPermission: (permissionIds: ReadonlyArray<PermissionId>) => boolean
  hasAllPermissions: (permissionIds: ReadonlyArray<PermissionId>) => boolean
  isLoading: boolean
  refetch: () => Promise<void>
}

export function usePermissions(): UsePermissionsResult {
  const currentUserQuery = useCurrentUser()
  const currentUser = currentUserQuery.data as AuthUser | undefined

  const query = useQuery<PermissionId[], ApiError>({
    queryKey: MY_PERMISSIONS_QUERY_KEY,
    queryFn: permissionsApi.getMyPermissions,
    enabled: Boolean(currentUser),
    initialData: (currentUser?.permissions ?? []) as PermissionId[],
    staleTime: 2 * 60 * 1000,
  })

  const permissionSet = useMemo(() => {
    const permissions = query.data ?? (currentUser?.permissions as PermissionId[] | undefined) ?? []
    return new Set<PermissionId>(permissions)
  }, [currentUser?.permissions, query.data])

  const hasPermission = useCallback(
    (permissionId: PermissionId | null | undefined) => {
      if (!permissionId) {
        return true
      }
      if (currentUser?.is_root) {
        return true
      }
      return permissionSet.has(permissionId)
    },
    [currentUser?.is_root, permissionSet]
  )

  const hasAnyPermission = useCallback(
    (permissionIds: ReadonlyArray<PermissionId>) => {
      if (!permissionIds.length) {
        return true
      }
      return permissionIds.some((id) => hasPermission(id))
    },
    [hasPermission]
  )

  const hasAllPermissions = useCallback(
    (permissionIds: ReadonlyArray<PermissionId>) => {
      if (!permissionIds.length) {
        return true
      }
      return permissionIds.every((id) => hasPermission(id))
    },
    [hasPermission]
  )

  const refetch = useCallback(async () => {
    await query.refetch()
  }, [query])

  return {
    permissions: Array.from(permissionSet),
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    isLoading: query.isLoading,
    refetch,
  }
}
