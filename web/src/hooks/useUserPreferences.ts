import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UserPreferences } from '@/types/preferences'
import type { ApiError } from '@/lib/api/http'
import { fetchUserPreferences, updateUserPreferences } from '@/lib/api/profile'

const QUERY_KEY = ['profile', 'preferences'] as const

export function useUserPreferences() {
  const queryClient = useQueryClient()

  const query = useQuery<UserPreferences, ApiError>({
    queryKey: QUERY_KEY,
    queryFn: fetchUserPreferences,
    staleTime: 5 * 60_000,
  })

  const mutation = useMutation<UserPreferences, ApiError, UserPreferences>({
    mutationFn: updateUserPreferences,
    onSuccess: (data) => {
      queryClient.setQueryData(QUERY_KEY, data)
    },
  })

  return {
    ...query,
    update: mutation,
  }
}
