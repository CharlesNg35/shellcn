import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchConnectionFolderTree } from '@/lib/api/connection-folders'
import type { ConnectionFolderNode } from '@/types/connections'
import { ApiError } from '@/lib/api/http'

export const CONNECTION_FOLDERS_QUERY_KEY = ['connections', 'folders', 'tree'] as const

export function useConnectionFolders(options?: UseQueryOptions<ConnectionFolderNode[], ApiError>) {
  return useQuery<ConnectionFolderNode[], ApiError>({
    queryKey: CONNECTION_FOLDERS_QUERY_KEY,
    queryFn: fetchConnectionFolderTree,
    staleTime: 60_000,
    ...options,
  })
}
