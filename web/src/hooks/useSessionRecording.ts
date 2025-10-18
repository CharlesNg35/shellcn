import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchSessionRecordingStatus, stopSessionRecording } from '@/lib/api/session-recordings'
import type { SessionRecordingStatus } from '@/types/session-recording'
import type { ApiError } from '@/lib/api/http'

export function useSessionRecording(sessionId: string | null, options?: { enabled?: boolean }) {
  const queryClient = useQueryClient()

  const query = useQuery<SessionRecordingStatus, ApiError>({
    queryKey: ['session-recording', sessionId],
    queryFn: () => fetchSessionRecordingStatus(sessionId!),
    enabled: Boolean(sessionId) && (options?.enabled ?? true),
    refetchInterval: (query) => (query.state.data?.active ? 2000 : 15000),
  })

  const mutation = useMutation<SessionRecordingStatus, ApiError, void>({
    mutationFn: () => stopSessionRecording(sessionId!),
    onSuccess: (data) => {
      queryClient.setQueryData(['session-recording', sessionId], data)
    },
  })

  return {
    status: query.data,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    refetch: query.refetch,
    stopRecording: mutation,
  }
}
