import { useQuery, useMutation, type UseMutationOptions } from '@tanstack/react-query'
import {
  fetchSnippets,
  executeSnippet,
  type FetchSnippetsParams,
  type SnippetRecord,
} from '@/lib/api/snippets'
import { ApiError } from '@/lib/api/http'

export interface UseSnippetsOptions extends FetchSnippetsParams {
  enabled?: boolean
}

const SNIPPETS_QUERY_KEY = ['snippets'] as const

export function useSnippets(options: UseSnippetsOptions = {}) {
  const { enabled = true, ...params } = options

  return useQuery<SnippetRecord[], ApiError>({
    queryKey: [...SNIPPETS_QUERY_KEY, params] as const,
    queryFn: () => fetchSnippets(params),
    enabled,
    staleTime: 60_000,
  })
}

export function useExecuteSnippet(
  options?: UseMutationOptions<void, ApiError, { sessionId: string; snippetId: string }>
) {
  return useMutation<void, ApiError, { sessionId: string; snippetId: string }>({
    mutationFn: ({ sessionId, snippetId }) => executeSnippet(sessionId, snippetId),
    ...options,
  })
}
