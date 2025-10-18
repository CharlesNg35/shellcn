import { useCallback } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryOptions,
} from '@tanstack/react-query'

import type { SessionParticipantsSummary, ActiveSessionParticipant } from '@/types/connections'
import {
  addSessionParticipant,
  fetchSessionParticipants,
  grantSessionParticipantWrite,
  relinquishSessionParticipantWrite,
  removeSessionParticipant,
  type AddSessionParticipantPayload,
  type RelinquishWriteResult,
} from '@/lib/api/session-participants'
import type { ApiError } from '@/lib/api/http'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import { ACTIVE_CONNECTIONS_QUERY_KEY } from './useActiveConnections'

export const SESSION_PARTICIPANTS_QUERY_KEY = (sessionId: string) =>
  ['active-sessions', sessionId, 'participants'] as const

type ParticipantsQueryOptions = Omit<
  UseQueryOptions<SessionParticipantsSummary, ApiError>,
  'queryKey' | 'queryFn'
>

export function useSessionParticipants(sessionId: string, options?: ParticipantsQueryOptions) {
  return useQuery<SessionParticipantsSummary, ApiError>({
    queryKey: SESSION_PARTICIPANTS_QUERY_KEY(sessionId),
    queryFn: () => fetchSessionParticipants(sessionId),
    enabled: Boolean(sessionId),
    staleTime: 10_000,
    ...options,
  })
}

interface ParticipantMutations {
  invite: UseMutationResult<ActiveSessionParticipant, ApiError, AddSessionParticipantPayload>
  remove: UseMutationResult<void, ApiError, { userId: string }>
  grantWrite: UseMutationResult<ActiveSessionParticipant, ApiError, { userId: string }>
  relinquishWrite: UseMutationResult<RelinquishWriteResult, ApiError, { userId: string }>
}

export function useSessionParticipantMutations(sessionId: string): ParticipantMutations {
  const queryClient = useQueryClient()

  const invalidate = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: SESSION_PARTICIPANTS_QUERY_KEY(sessionId) }),
      queryClient.invalidateQueries({ queryKey: ACTIVE_CONNECTIONS_QUERY_KEY }),
    ])
  }, [queryClient, sessionId])

  const invite = useMutation<ActiveSessionParticipant, ApiError, AddSessionParticipantPayload>({
    mutationFn: (payload) => addSessionParticipant(sessionId, payload),
    onSuccess: async (participant) => {
      await invalidate()
      toast.success('Participant added', {
        description: participant.user_name
          ? `${participant.user_name} can now view this session`
          : 'Participant can now view this session',
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Unable to add participant', {
        description: apiError.message,
      })
    },
  })

  const remove = useMutation<void, ApiError, { userId: string }>({
    mutationFn: ({ userId }) => removeSessionParticipant(sessionId, userId),
    onSuccess: async () => {
      await invalidate()
      toast.success('Participant removed')
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Unable to remove participant', {
        description: apiError.message,
      })
    },
  })

  const grantWrite = useMutation<ActiveSessionParticipant, ApiError, { userId: string }>({
    mutationFn: ({ userId }) => grantSessionParticipantWrite(sessionId, userId),
    onSuccess: async (participant) => {
      await invalidate()
      toast.success('Write access granted', {
        description: participant.user_name
          ? `${participant.user_name} can now control the terminal`
          : 'Write access granted',
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Unable to grant write access', {
        description: apiError.message,
      })
    },
  })

  const relinquishWrite = useMutation<RelinquishWriteResult, ApiError, { userId: string }>({
    mutationFn: ({ userId }) => relinquishSessionParticipantWrite(sessionId, userId),
    onSuccess: async () => {
      await invalidate()
      toast.success('Write access relinquished')
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Unable to relinquish write access', {
        description: apiError.message,
      })
    },
  })

  return { invite, remove, grantWrite, relinquishWrite }
}
