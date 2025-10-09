import { useCallback, useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
  type UseQueryOptions,
} from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
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
    onSuccess: async () => {
      await invalidateUsers()
    },
  })

  const update = useMutation({
    mutationFn: ({ userId, payload }: { userId: string; payload: UserUpdatePayload }) =>
      updateUser(userId, payload),
    onSuccess: async (result) => {
      await invalidateUsers()
      await invalidateUser(result?.id)
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
    onSuccess: async () => {
      await invalidateUsers()
    },
  })

  const bulkDeactivate = useMutation({
    mutationFn: (payload: BulkUserPayload) => bulkDeactivateUsers(payload.user_ids),
    onSuccess: async () => {
      await invalidateUsers()
    },
  })

  const bulkDelete = useMutation({
    mutationFn: (payload: BulkUserPayload) => bulkDeleteUsers(payload.user_ids),
    onSuccess: async () => {
      await invalidateUsers()
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
