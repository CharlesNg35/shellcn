import { useCallback } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query'
import { toApiError, type ApiError } from '@/lib/api/http'
import {
  createRole as createRoleApi,
  deleteRole as deleteRoleApi,
  permissionsApi,
  setRolePermissions as setRolePermissionsApi,
  updateRole as updateRoleApi,
} from '@/lib/api/permissions'
import type {
  RoleCreatePayload,
  RoleRecord,
  RoleUpdatePayload,
  SetRolePermissionsPayload,
} from '@/types/permission'
import { toast } from '@/lib/utils/toast'

export const ROLES_QUERY_KEY = ['permissions', 'roles'] as const

type RolesQueryOptions = Omit<
  UseQueryOptions<RoleRecord[], ApiError, RoleRecord[], typeof ROLES_QUERY_KEY>,
  'queryKey' | 'queryFn'
>

export function useRoles(options?: RolesQueryOptions): UseQueryResult<RoleRecord[], ApiError> {
  return useQuery({
    queryKey: ROLES_QUERY_KEY,
    queryFn: permissionsApi.listRoles,
    staleTime: 30 * 1000,
    placeholderData: (previous) => previous,
    ...options,
  })
}

type DeleteRoleVariables = {
  roleId: string
  roleName?: string
}

type SetRolePermissionsVariables = {
  roleId: string
  payload: SetRolePermissionsPayload
  roleName?: string
}

export function useRoleMutations() {
  const queryClient = useQueryClient()

  const invalidateRoles = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ROLES_QUERY_KEY })
  }, [queryClient])

  const createRole = useMutation({
    mutationFn: (payload: RoleCreatePayload) => createRoleApi(payload),
    onSuccess: async (role) => {
      await invalidateRoles()
      toast.success('Role created', {
        description: `${role.name} is now available for assignment`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to create role', {
        description: apiError.message || 'Please try again later',
      })
    },
  })

  const updateRole = useMutation({
    mutationFn: ({ roleId, payload }: { roleId: string; payload: RoleUpdatePayload }) =>
      updateRoleApi(roleId, payload),
    onSuccess: async (role) => {
      await invalidateRoles()
      toast.success('Role updated', {
        description: `${role.name} saved successfully`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to update role', {
        description: apiError.message || 'Please try again later',
      })
    },
  })

  const deleteRole = useMutation<boolean, ApiError, DeleteRoleVariables>({
    mutationFn: ({ roleId }) => deleteRoleApi(roleId),
    onSuccess: async (_, variables) => {
      await invalidateRoles()
      toast.success('Role deleted', {
        description: variables.roleName
          ? `${variables.roleName} has been removed`
          : `Role ${variables.roleId} has been removed`,
      })
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to delete role', {
        description: apiError.message || 'Please try again later',
      })
    },
  })

  const setRolePermissions = useMutation<boolean, ApiError, SetRolePermissionsVariables>({
    mutationFn: ({ roleId, payload }) => setRolePermissionsApi(roleId, payload),
    onSuccess: async (_, variables) => {
      await invalidateRoles()
      toast.success('Permissions updated', {
        description: variables.roleName
          ? `${variables.roleName} permissions have been saved`
          : 'Role permissions have been saved',
      })
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to update permissions', {
        description: apiError.message || 'Please try again later',
      })
    },
  })

  return {
    createRole,
    updateRole,
    deleteRole,
    setRolePermissions,
  }
}
