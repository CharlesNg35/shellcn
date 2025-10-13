import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { invitesApi } from '@/lib/api/invites'
import { toApiError, type ApiError } from '@/lib/api/http'
import type { InviteCreatePayload, InviteRecord } from '@/types/invites'
import { toast } from '@/lib/utils/toast'

export const INVITES_QUERY_KEY = ['invites'] as const

export function useInvites() {
  return useQuery<InviteRecord[], ApiError>({
    queryKey: INVITES_QUERY_KEY,
    queryFn: () => invitesApi.list(),
    staleTime: 30_000,
  })
}

export function useInviteMutations() {
  const queryClient = useQueryClient()

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: INVITES_QUERY_KEY })
  }

  const create = useMutation({
    mutationFn: (payload: InviteCreatePayload) => invitesApi.create(payload),
    onSuccess: async (_, variables) => {
      await invalidate()
      toast.success('Invitation created', {
        description: variables.team_id
          ? 'User will be added to the selected team on acceptance'
          : undefined,
      })
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to create invitation', {
        description: apiError.message,
      })
    },
  })

  const remove = useMutation({
    mutationFn: (inviteId: string) => invitesApi.delete(inviteId),
    onSuccess: async () => {
      await invalidate()
      toast.success('Invitation revoked')
    },
    onError: (error) => {
      const apiError = toApiError(error)
      toast.error('Failed to revoke invitation', {
        description: apiError.message,
      })
    },
  })

  return { create, remove, invalidate }
}
