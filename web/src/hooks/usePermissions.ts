import { useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { permissionsApi } from '@/lib/api/permissions'
import { useCurrentUser } from './useCurrentUser'
import type { AuthUser } from '@/types/auth'

export const MY_PERMISSIONS_QUERY_KEY = ['permissions', 'my'] as const

export interface UsePermissionsResult {
  permissions: string[]
  hasPermission: (permissionId: string) => boolean
  hasAnyPermission: (permissionIds: string[]) => boolean
  hasAllPermissions: (permissionIds: string[]) => boolean
  isLoading: boolean
  refetch: () => Promise<void>
}

export function usePermissions(): UsePermissionsResult {
  const currentUserQuery = useCurrentUser()
  const currentUser = currentUserQuery.data as AuthUser | undefined

  const query = useQuery<string[], ApiError>({
    queryKey: MY_PERMISSIONS_QUERY_KEY,
    queryFn: permissionsApi.getMyPermissions,
    enabled: Boolean(currentUser),
    initialData: currentUser?.permissions ?? [],
    staleTime: 2 * 60 * 1000,
  })

  const permissionSet = useMemo(
    () => new Set(query.data ?? currentUser?.permissions ?? []),
    [currentUser?.permissions, query.data]
  )

  const hasPermission = useCallback(
    (permissionId: string) => {
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
    (permissionIds: string[]) => {
      if (!permissionIds.length) {
        return true
      }
      return permissionIds.some((id) => hasPermission(id))
    },
    [hasPermission]
  )

  const hasAllPermissions = useCallback(
    (permissionIds: string[]) => {
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
