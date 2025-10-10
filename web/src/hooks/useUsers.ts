import { useCallback, useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { toApiError, type ApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import {
  activateUser,
  bulkActivateUsers,
  bulkDeactivateUsers,
  bulkDeleteUsers,
  changeUserPassword,
  createUser,
  deactivateUser,
  fetchUserById,
  fetchUsers,
  updateUser,
} from '@/lib/api/users'
import type {
  BulkUserPayload,
  UserCreatePayload,
  UserListParams,
  UserListResult,
  UserRecord,
  UserUpdatePayload,
} from '@/types/users'

export const USERS_LIST_QUERY_KEY = ['users', 'list'] as const

export function getUsersQueryKey(params?: UserListParams) {
  return [...USERS_LIST_QUERY_KEY, params ?? {}] as const
}

export const USER_DETAIL_QUERY_KEY = ['users', 'detail'] as const

export function getUserDetailQueryKey(userId?: string) {
  return [...USER_DETAIL_QUERY_KEY, userId ?? ''] as const
}

type UsersQueryOptions = Omit<
  UseQueryOptions<UserListResult, ApiError, UserListResult, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

type UserDetailQueryOptions = Omit<
  UseQueryOptions<UserRecord, ApiError, UserRecord, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

export function useUsers(
  params?: UserListParams,
  options?: UsersQueryOptions
): UseQueryResult<UserListResult, ApiError> {
  const queryKey = useMemo(() => getUsersQueryKey(params), [params])

  return useQuery<UserListResult, ApiError, UserListResult, readonly unknown[]>({
    queryKey,
    queryFn: () => fetchUsers(params),
    placeholderData: (prev) => prev ?? undefined,
    staleTime: 60_000,
    ...(options ?? {}),
  })
}

export function useUser(
  userId: string | undefined,
  options?: UserDetailQueryOptions
): UseQueryResult<UserRecord, ApiError> {
  const queryKey = useMemo(() => getUserDetailQueryKey(userId), [userId])

  return useQuery<UserRecord, ApiError, UserRecord, readonly unknown[]>({
    queryKey,
    queryFn: () => fetchUserById(userId as string),
    enabled: Boolean(userId),
    staleTime: 60_000,
    ...(options ?? {}),
  })
}

export function useUserMutations() {
  const queryClient = useQueryClient()

  const invalidateUsers = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: USERS_LIST_QUERY_KEY })
  }, [queryClient])

  const invalidateUser = useCallback(
    async (userId?: string) => {
      if (!userId) {
        return
      }
      await queryClient.invalidateQueries({ queryKey: getUserDetailQueryKey(userId) })
    },
    [queryClient]
  )

  const create = useMutation({
    mutationFn: (payload: UserCreatePayload) => createUser(payload),
    onSuccess: async (user) => {
      await invalidateUsers()
      toast.success('User created successfully', {
        description: `${user.username} has been added to the system`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to create user', {
        description: apiError.message || 'Please try again',
      })
    },
  })

  const update = useMutation({
    mutationFn: ({ userId, payload }: { userId: string; payload: UserUpdatePayload }) =>
      updateUser(userId, payload),
    onSuccess: async (result) => {
      await invalidateUsers()
      await invalidateUser(result?.id)
      toast.success('User updated successfully')
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to update user', {
        description: apiError.message || 'Please try again',
      })
    },
  })

  const activate = useMutation({
    mutationFn: (userId: string) => activateUser(userId),
    onSuccess: async (result) => {
      await invalidateUsers()
      await invalidateUser(result.id)
    },
  })

  const deactivate = useMutation({
    mutationFn: (userId: string) => deactivateUser(userId),
    onSuccess: async (result) => {
      await invalidateUsers()
      await invalidateUser(result.id)
    },
  })

  const changePasswordMutation = useMutation({
    mutationFn: ({ userId, password }: { userId: string; password: string }) =>
      changeUserPassword(userId, password),
  })

  const bulkActivate = useMutation({
    mutationFn: (payload: BulkUserPayload) => bulkActivateUsers(payload.user_ids),
    onSuccess: async (_, variables) => {
      await invalidateUsers()
      toast.success('Users activated', {
        description: `${variables.user_ids.length} user(s) activated successfully`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to activate users', {
        description: apiError.message || 'Please try again',
      })
    },
  })

  const bulkDeactivate = useMutation({
    mutationFn: (payload: BulkUserPayload) => bulkDeactivateUsers(payload.user_ids),
    onSuccess: async (_, variables) => {
      await invalidateUsers()
      toast.success('Users deactivated', {
        description: `${variables.user_ids.length} user(s) deactivated successfully`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to deactivate users', {
        description: apiError.message || 'Please try again',
      })
    },
  })

  const bulkDelete = useMutation({
    mutationFn: (payload: BulkUserPayload) => bulkDeleteUsers(payload.user_ids),
    onSuccess: async (_, variables) => {
      await invalidateUsers()
      toast.success('Users deleted', {
        description: `${variables.user_ids.length} user(s) deleted successfully`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to delete users', {
        description: apiError.message || 'Please try again',
      })
    },
  })

  return {
    create,
    update,
    activate,
    deactivate,
    changePassword: changePasswordMutation,
    bulkActivate,
    bulkDeactivate,
    bulkDelete,
  }
}
