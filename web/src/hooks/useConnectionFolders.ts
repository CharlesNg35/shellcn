import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchConnectionFolderTree } from '@/lib/api/connection-folders'
import type { ConnectionFolderNode } from '@/types/connections'
import { ApiError } from '@/lib/api/http'

export const CONNECTION_FOLDERS_QUERY_KEY = ['connections', 'folders', 'tree'] as const

type FolderQueryOptions = Omit<
  UseQueryOptions<ConnectionFolderNode[], ApiError>,
  'queryKey' | 'queryFn'
>

export function useConnectionFolders(options?: FolderQueryOptions) {
  return useQuery<ConnectionFolderNode[], ApiError>({
    queryKey: CONNECTION_FOLDERS_QUERY_KEY,
    queryFn: fetchConnectionFolderTree,
    staleTime: 60_000,
    ...options,
  })
}
