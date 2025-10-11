import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createConnectionFolder,
  deleteConnectionFolder,
  updateConnectionFolder,
  type UpsertFolderPayload,
} from '@/lib/api/connection-folders'
import { toast } from '@/lib/utils/toast'
import { toApiError } from '@/lib/api/http'
import { CONNECTION_FOLDERS_QUERY_KEY } from './useConnectionFolders'
import { CONNECTIONS_QUERY_BASE_KEY } from './useConnections'

function useInvalidateFolders() {
  const queryClient = useQueryClient()

  return useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: CONNECTION_FOLDERS_QUERY_KEY })
    await queryClient.invalidateQueries({ queryKey: CONNECTIONS_QUERY_BASE_KEY })
  }, [queryClient])
}

export function useConnectionFolderMutations() {
  const invalidateFolders = useInvalidateFolders()

  const create = useMutation({
    mutationFn: (payload: UpsertFolderPayload) => createConnectionFolder(payload),
    onSuccess: async (folder) => {
      await invalidateFolders()
      toast.success('Folder created', {
        description: `${folder.name} is ready to organize your connections.`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to create folder', {
        description: apiError.message,
      })
    },
  })

  const update = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpsertFolderPayload }) =>
      updateConnectionFolder(id, payload),
    onSuccess: async (folder) => {
      await invalidateFolders()
      toast.success('Folder updated', {
        description: `${folder.name} saved successfully.`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to update folder', {
        description: apiError.message,
      })
    },
  })

  const remove = useMutation({
    mutationFn: (folderId: string) => deleteConnectionFolder(folderId),
    onSuccess: async () => {
      await invalidateFolders()
      toast.success('Folder deleted')
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to delete folder', {
        description: apiError.message,
      })
    },
  })

  return {
    create,
    update,
    remove,
  }
}
