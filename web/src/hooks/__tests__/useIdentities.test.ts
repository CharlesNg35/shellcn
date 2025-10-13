import { createElement, type ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getIdentitiesQueryKey,
  getIdentityDetailQueryKey,
  useIdentities,
  useIdentityMutations,
  useIdentitySharing,
  VAULT_IDENTITIES_QUERY_KEY,
} from '@/hooks/useIdentities'
import type { IdentityRecord } from '@/types/vault'
import { toast } from '@/lib/utils/toast'

const mockFetchIdentities = vi.fn()
const mockCreateIdentity = vi.fn()
const mockUpdateIdentity = vi.fn()
const mockDeleteIdentity = vi.fn()
const mockFetchIdentity = vi.fn()
const mockFetchTemplates = vi.fn()
const mockCreateShare = vi.fn()
const mockDeleteShare = vi.fn()

vi.mock('@/lib/api/vault', () => ({
  fetchIdentities: (params: unknown) => mockFetchIdentities(params),
  fetchIdentity: (id: string, options?: unknown) => mockFetchIdentity(id, options),
  fetchCredentialTemplates: () => mockFetchTemplates(),
  createIdentity: (payload: unknown) => mockCreateIdentity(payload),
  updateIdentity: (id: string, payload: unknown) => mockUpdateIdentity(id, payload),
  deleteIdentity: (id: string) => mockDeleteIdentity(id),
  createIdentityShare: (id: string, payload: unknown) => mockCreateShare(id, payload),
  deleteIdentityShare: (shareId: string) => mockDeleteShare(shareId),
}))

vi.mock('@/lib/utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
    promise: vi.fn(),
    custom: vi.fn(),
  },
}))

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  })
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient, children })
}

const baseIdentity: IdentityRecord = {
  id: 'id-1',
  name: 'Root SSH Key',
  description: 'Shared key',
  scope: 'global',
  owner_user_id: 'usr-1',
  version: 1,
  metadata: {},
  usage_count: 0,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  connection_count: 0,
  team_id: null,
  connection_id: null,
  template_id: null,
  shares: [],
}

describe('useIdentities hooks', () => {
  beforeEach(() => {
    mockFetchIdentities.mockReset()
    mockCreateIdentity.mockReset()
    mockUpdateIdentity.mockReset()
    mockDeleteIdentity.mockReset()
    mockFetchIdentity.mockReset()
    mockFetchTemplates.mockReset()
    mockCreateShare.mockReset()
    mockDeleteShare.mockReset()

    Object.values(toast).forEach((fn) => {
      if ('mockReset' in fn) {
        ;(fn as unknown as { mockReset: () => void }).mockReset()
      }
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('fetches identities via useIdentities', async () => {
    const identities = [baseIdentity]
    mockFetchIdentities.mockResolvedValueOnce(identities)

    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)

    const { result } = renderHook(() => useIdentities(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(identities)
    expect(mockFetchIdentities).toHaveBeenCalledTimes(1)
    queryClient.clear()
  })

  it('triggers success toast and invalidation when creating an identity', async () => {
    const createdIdentity: IdentityRecord = {
      ...baseIdentity,
      id: 'id-2',
      name: 'Database Credential',
    }
    mockCreateIdentity.mockResolvedValueOnce(createdIdentity)

    const queryClient = createQueryClient()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = createWrapper(queryClient)

    const { result } = renderHook(() => useIdentityMutations(), { wrapper })

    await act(async () => {
      await result.current.create.mutateAsync({
        name: createdIdentity.name,
        scope: createdIdentity.scope,
        payload: {},
      })
    })

    expect(mockCreateIdentity).toHaveBeenCalledWith({
      name: createdIdentity.name,
      scope: createdIdentity.scope,
      payload: {},
    })
    expect(toast.success).toHaveBeenCalledWith('Identity created', expect.any(Object))
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: VAULT_IDENTITIES_QUERY_KEY,
    })
    queryClient.clear()
  })

  it('restores cache and emits error toast when deletion fails', async () => {
    mockDeleteIdentity.mockRejectedValueOnce(new Error('server error'))

    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)

    queryClient.setQueryData(getIdentitiesQueryKey(), [baseIdentity])

    const { result } = renderHook(() => useIdentityMutations(), { wrapper })

    await act(async () => {
      await expect(result.current.remove.mutateAsync(baseIdentity.id)).rejects.toThrowError()
    })

    const cached = queryClient.getQueryData<IdentityRecord[]>(getIdentitiesQueryKey())
    expect(cached).toEqual([baseIdentity])
    expect(toast.error).toHaveBeenCalledWith('Failed to delete identity', expect.any(Object))
    queryClient.clear()
  })

  it('performs optimistic share grant and reverts on failure', async () => {
    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)
    const identityDetail: IdentityRecord = {
      ...baseIdentity,
      shares: [],
    }

    const detailKey = getIdentityDetailQueryKey(identityDetail.id)
    queryClient.setQueryData(detailKey, identityDetail)

    mockCreateShare.mockRejectedValueOnce(new Error('denied'))

    const { result } = renderHook(() => useIdentitySharing(identityDetail.id), { wrapper })

    await act(async () => {
      await expect(
        result.current.grant.mutateAsync({
          principal_type: 'user',
          principal_id: 'usr-2',
          permission: 'use',
        })
      ).rejects.toThrowError()
    })

    const restored = queryClient.getQueryData<IdentityRecord>(detailKey)
    expect(restored?.shares).toHaveLength(0)
    expect(toast.error).toHaveBeenCalledWith('Failed to share identity', expect.any(Object))
    queryClient.clear()
  })

  it('adds share optimistically on success and shows success toast', async () => {
    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)
    const identityDetail: IdentityRecord = {
      ...baseIdentity,
      shares: [],
    }

    const detailKey = getIdentityDetailQueryKey(identityDetail.id)
    queryClient.setQueryData(detailKey, identityDetail)

    mockCreateShare.mockResolvedValueOnce({
      id: 'share-1',
      principal_type: 'user',
      principal_id: 'usr-2',
      permission: 'use',
      granted_by: 'usr-1',
      created_by: 'usr-1',
      metadata: null,
      revoked_at: null,
      revoked_by: null,
      expires_at: null,
    })

    const { result } = renderHook(() => useIdentitySharing(identityDetail.id), { wrapper })

    const mutatePromise = result.current.grant.mutateAsync({
      principal_type: 'user',
      principal_id: 'usr-2',
      permission: 'use',
    })

    await waitFor(() => {
      const optimistic = queryClient.getQueryData<IdentityRecord>(detailKey)
      expect(optimistic?.shares).toHaveLength(1)
    })

    await act(async () => {
      await mutatePromise
    })

    expect(mockCreateShare).toHaveBeenCalledTimes(1)
    expect(toast.success).toHaveBeenCalledWith(
      'Access granted',
      expect.objectContaining({
        description: 'The identity has been shared successfully.',
      })
    )

    queryClient.clear()
  })
})
