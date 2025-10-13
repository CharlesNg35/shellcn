import { useQuery, type UseQueryOptions, type UseQueryResult } from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { permissionsApi } from '@/lib/api/permissions'
import type { PermissionRegistry } from '@/types/permission'

export const PERMISSION_REGISTRY_QUERY_KEY = ['permissions', 'registry'] as const

type PermissionRegistryQueryOptions = Omit<
  UseQueryOptions<
    PermissionRegistry,
    ApiError,
    PermissionRegistry,
    typeof PERMISSION_REGISTRY_QUERY_KEY
  >,
  'queryKey' | 'queryFn'
>

export function usePermissionRegistry(
  options?: PermissionRegistryQueryOptions
): UseQueryResult<PermissionRegistry, ApiError> {
  return useQuery({
    queryKey: PERMISSION_REGISTRY_QUERY_KEY,
    queryFn: permissionsApi.getRegistry,
    staleTime: 5 * 60 * 1000,
    ...options,
  })
}
