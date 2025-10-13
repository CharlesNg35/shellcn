import { useCallback, useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query'
import {
  createIdentity,
  createIdentityShare,
  deleteIdentity,
  deleteIdentityShare,
  fetchCredentialTemplates,
  fetchIdentities,
  fetchIdentity,
  updateIdentity,
} from '@/lib/api/vault'
import { toApiError, type ApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import type {
  CredentialTemplateRecord,
  IdentityCreatePayload,
  IdentityListParams,
  IdentityRecord,
  IdentitySharePayload,
  IdentityShareRecord,
  IdentityUpdatePayload,
} from '@/types/vault'

export const VAULT_IDENTITIES_QUERY_KEY = ['vault', 'identities'] as const

export function getIdentitiesQueryKey(params?: IdentityListParams) {
  return [...VAULT_IDENTITIES_QUERY_KEY, params ?? {}] as const
}

export const VAULT_IDENTITY_DETAIL_QUERY_KEY = ['vault', 'identity'] as const

export function getIdentityDetailQueryKey(identityId?: string) {
  return [...VAULT_IDENTITY_DETAIL_QUERY_KEY, identityId ?? ''] as const
}

export const VAULT_TEMPLATES_QUERY_KEY = ['vault', 'templates'] as const

type IdentitiesQueryOptions = Omit<
  UseQueryOptions<IdentityRecord[], ApiError, IdentityRecord[], readonly unknown[]>,
  'queryKey' | 'queryFn'
>

type IdentityDetailQueryOptions = Omit<
  UseQueryOptions<IdentityRecord, ApiError, IdentityRecord, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

type TemplateQueryOptions = Omit<
  UseQueryOptions<
    CredentialTemplateRecord[],
    ApiError,
    CredentialTemplateRecord[],
    readonly unknown[]
  >,
  'queryKey' | 'queryFn'
>

export function useIdentities(
  params?: IdentityListParams,
  options?: IdentitiesQueryOptions
): UseQueryResult<IdentityRecord[], ApiError> {
  const queryKey = useMemo(() => getIdentitiesQueryKey(params), [params])

  return useQuery<IdentityRecord[], ApiError, IdentityRecord[], readonly unknown[]>({
    queryKey,
    queryFn: () => fetchIdentities(params),
    staleTime: 30_000,
    placeholderData: (previousData) => previousData ?? undefined,
    ...(options ?? {}),
  })
}

export interface UseIdentityOptions extends IdentityDetailQueryOptions {
  includePayload?: boolean
}

export function useIdentity(
  identityId: string | undefined,
  options?: UseIdentityOptions
): UseQueryResult<IdentityRecord, ApiError> {
  const queryKey = useMemo(() => getIdentityDetailQueryKey(identityId), [identityId])

  return useQuery<IdentityRecord, ApiError, IdentityRecord, readonly unknown[]>({
    queryKey,
    queryFn: () => fetchIdentity(identityId as string, { includePayload: options?.includePayload }),
    enabled: Boolean(identityId),
    staleTime: 30_000,
    ...(options ?? {}),
  })
}

export function useIdentityMutations(identityIdForDetailInvalidate?: string) {
  const queryClient = useQueryClient()

  type RemoveContext = {
    previousEntries: Array<{
      queryKey: readonly unknown[]
      data?: IdentityRecord[]
    }>
  }

  const invalidateLists = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: VAULT_IDENTITIES_QUERY_KEY })
  }, [queryClient])

  const invalidateDetail = useCallback(async () => {
    if (!identityIdForDetailInvalidate) {
      return
    }
    await queryClient.invalidateQueries({
      queryKey: getIdentityDetailQueryKey(identityIdForDetailInvalidate),
    })
  }, [identityIdForDetailInvalidate, queryClient])

  const create = useMutation<IdentityRecord, unknown, IdentityCreatePayload>({
    mutationFn: (payload: IdentityCreatePayload) => createIdentity(payload),
    onSuccess: async (identity) => {
      await invalidateLists()
      toast.success('Identity created', {
        description: `${identity.name} is ready to use.`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to create identity', {
        description: apiError.message,
      })
    },
  })

  const update = useMutation<
    IdentityRecord,
    unknown,
    { identityId: string; payload: IdentityUpdatePayload }
  >({
    mutationFn: ({ identityId, payload }: { identityId: string; payload: IdentityUpdatePayload }) =>
      updateIdentity(identityId, payload),
    onSuccess: async (identity) => {
      await invalidateLists()
      await queryClient.invalidateQueries({
        queryKey: getIdentityDetailQueryKey(identity.id),
      })
      toast.success('Identity updated', {
        description: 'Credential metadata saved successfully.',
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to update identity', {
        description: apiError.message,
      })
    },
  })

  const remove = useMutation<void, unknown, string, RemoveContext>({
    mutationFn: (identityId: string) => deleteIdentity(identityId),
    onMutate: async (identityId: string) => {
      await queryClient.cancelQueries({ queryKey: VAULT_IDENTITIES_QUERY_KEY })

      const listKeys = queryClient.getQueryCache().findAll({ queryKey: VAULT_IDENTITIES_QUERY_KEY })

      const previousEntries = listKeys.map((entry) => ({
        queryKey: entry.queryKey,
        data: entry.state.data as IdentityRecord[] | undefined,
      }))

      previousEntries.forEach(({ queryKey: key, data }) => {
        if (!data) {
          return
        }
        queryClient.setQueryData(
          key,
          data.filter((identity) => identity.id !== identityId)
        )
      })

      return { previousEntries }
    },
    onError: (error: unknown, _identityId, context) => {
      context?.previousEntries?.forEach(({ queryKey, data }) => {
        if (data) {
          queryClient.setQueryData(queryKey, data)
        }
      })

      const apiError = toApiError(error)
      toast.error('Failed to delete identity', {
        description: apiError.message,
      })
    },
    onSuccess: async () => {
      toast.success('Identity deleted', {
        description: 'The credential has been removed.',
      })
    },
    onSettled: async () => {
      await invalidateLists()
      await invalidateDetail()
    },
  })

  return {
    create,
    update,
    remove,
    invalidateLists,
    invalidateDetail,
  }
}

export function useCredentialTemplates(options?: TemplateQueryOptions) {
  return useQuery<
    CredentialTemplateRecord[],
    ApiError,
    CredentialTemplateRecord[],
    readonly unknown[]
  >({
    queryKey: VAULT_TEMPLATES_QUERY_KEY,
    queryFn: () => fetchCredentialTemplates(),
    staleTime: 60_000,
    ...(options ?? {}),
  })
}

export function useIdentitySharing(identityId: string | undefined) {
  const queryClient = useQueryClient()

  const detailQueryKey = identityId ? getIdentityDetailQueryKey(identityId) : null

  type ShareContext = {
    previous?: IdentityRecord
  }

  const grant = useMutation<IdentityShareRecord, unknown, IdentitySharePayload, ShareContext>({
    mutationFn: (payload: IdentitySharePayload) =>
      createIdentityShare(identityId as string, payload),
    onMutate: async (payload: IdentitySharePayload) => {
      if (!detailQueryKey) {
        return {}
      }

      await queryClient.cancelQueries({ queryKey: detailQueryKey })

      const previous = queryClient.getQueryData<IdentityRecord>(detailQueryKey)

      if (previous) {
        const optimisticShare: IdentityShareRecord = {
          id: `optimistic-${Date.now()}`,
          principal_type: payload.principal_type,
          principal_id: payload.principal_id,
          permission: payload.permission,
          expires_at: payload.expires_at ?? null,
          metadata: payload.metadata ?? undefined,
          granted_by: previous.owner_user_id,
          created_by: previous.owner_user_id,
          revoked_by: null,
          revoked_at: null,
        }

        queryClient.setQueryData(detailQueryKey, {
          ...previous,
          shares: [...(previous.shares ?? []), optimisticShare],
        })
      }

      return { previous }
    },
    onError: (error: unknown, _payload, context) => {
      if (detailQueryKey && context?.previous) {
        queryClient.setQueryData(detailQueryKey, context.previous)
      }
      const apiError = toApiError(error)
      toast.error('Failed to share identity', {
        description: apiError.message,
      })
    },
    onSuccess: async () => {
      toast.success('Access granted', {
        description: 'The identity has been shared successfully.',
      })
    },
    onSettled: async () => {
      if (detailQueryKey) {
        await queryClient.invalidateQueries({ queryKey: detailQueryKey })
      }
    },
  })

  const revoke = useMutation<void, unknown, string, ShareContext>({
    mutationFn: (shareId: string) => deleteIdentityShare(shareId),
    onMutate: async (shareId: string) => {
      if (!detailQueryKey) {
        return {}
      }

      await queryClient.cancelQueries({ queryKey: detailQueryKey })

      const previous = queryClient.getQueryData<IdentityRecord>(detailQueryKey)

      if (previous) {
        queryClient.setQueryData(detailQueryKey, {
          ...previous,
          shares: (previous.shares ?? []).filter((share) => share.id !== shareId),
        })
      }

      return { previous }
    },
    onError: (error: unknown, _shareId, context) => {
      if (detailQueryKey && context?.previous) {
        queryClient.setQueryData(detailQueryKey, context.previous)
      }
      const apiError = toApiError(error)
      toast.error('Failed to revoke access', {
        description: apiError.message,
      })
    },
    onSuccess: async () => {
      toast.success('Access revoked')
    },
    onSettled: async () => {
      if (detailQueryKey) {
        await queryClient.invalidateQueries({ queryKey: detailQueryKey })
      }
    },
  })

  return {
    grant,
    revoke,
  }
}
