import { useMemo } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { fetchConnectionFolderTree } from '@/lib/api/connection-folders'
import type { ConnectionFolderNode } from '@/types/connections'
import { ApiError } from '@/lib/api/http'

export const CONNECTION_FOLDERS_QUERY_KEY = ['connections', 'folders', 'tree'] as const

type FolderQueryOptions = Omit<
  UseQueryOptions<ConnectionFolderNode[], ApiError>,
  'queryKey' | 'queryFn'
>

export function useConnectionFolders(teamId?: string, options?: FolderQueryOptions) {
  const queryKey = useMemo(
    () => [...CONNECTION_FOLDERS_QUERY_KEY, teamId ?? 'all'] as const,
    [teamId]
  )

  return useQuery<ConnectionFolderNode[], ApiError>({
    queryKey,
    queryFn: () => fetchConnectionFolderTree(teamId),
    staleTime: 60_000,
    ...options,
  })
}
