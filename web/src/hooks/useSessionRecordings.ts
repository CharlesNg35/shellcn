import { useMemo } from 'react'
import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query'

import {
  deleteSessionRecording,
  fetchSessionRecordings,
  type FetchSessionRecordingsParams,
} from '@/lib/api/session-recordings'
import { ApiError } from '@/lib/api/http'
import type { SessionRecordingSummary } from '@/types/session-recording'
import type { ApiMeta } from '@/types/api'

type RecordingQueryResult = {
  data: SessionRecordingSummary[]
  meta?: ApiMeta
}

export const SESSION_RECORDINGS_QUERY_KEY = ['session-recordings'] as const

export interface UseSessionRecordingsOptions extends FetchSessionRecordingsParams {
  enabled?: boolean
}

type QueryOptions = Omit<UseQueryOptions<RecordingQueryResult, ApiError>, 'queryKey' | 'queryFn'>

type DeleteOptions = UseMutationOptions<void, ApiError, string>

export function useSessionRecordings(
  options: UseSessionRecordingsOptions = {},
  queryOptions?: QueryOptions
) {
  const { enabled = true, ...params } = options
  const {
    protocol_id,
    connection_id,
    scope,
    team_id,
    page,
    per_page,
    sort,
    owner_user_id,
    created_by_user_id,
  } = params

  const queryKey = useMemo(
    () =>
      [
        ...SESSION_RECORDINGS_QUERY_KEY,
        {
          protocol_id,
          connection_id,
          scope,
          team_id,
          page,
          per_page,
          sort,
          owner_user_id,
          created_by_user_id,
        },
      ] as const,
    [
      protocol_id,
      connection_id,
      scope,
      team_id,
      page,
      per_page,
      sort,
      owner_user_id,
      created_by_user_id,
    ]
  )

  return useQuery<RecordingQueryResult, ApiError>({
    queryKey,
    queryFn: () => fetchSessionRecordings(params),
    enabled,
    ...queryOptions,
  })
}

export function useDeleteSessionRecording(options?: DeleteOptions) {
  return useMutation<void, ApiError, string>({
    mutationKey: [...SESSION_RECORDINGS_QUERY_KEY, 'delete'],
    mutationFn: async (recordId: string) => {
      await deleteSessionRecording(recordId)
    },
    ...options,
  })
}
