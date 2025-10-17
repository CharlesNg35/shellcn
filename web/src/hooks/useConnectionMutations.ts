import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createConnection,
  updateConnection,
  deleteConnection,
  type ConnectionCreatePayload,
  type ConnectionUpdatePayload,
} from '@/lib/api/connections'
import { toast } from '@/lib/utils/toast'
import { toApiError } from '@/lib/api/http'
import { CONNECTIONS_QUERY_BASE_KEY } from './useConnections'
import { CONNECTION_FOLDERS_QUERY_KEY } from './useConnectionFolders'

export function useConnectionMutations() {
  const queryClient = useQueryClient()

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: CONNECTIONS_QUERY_BASE_KEY }),
      queryClient.invalidateQueries({ queryKey: CONNECTION_FOLDERS_QUERY_KEY }),
    ])
  }

  const create = useMutation({
    mutationFn: (payload: ConnectionCreatePayload) => createConnection(payload),
    onSuccess: async (connection) => {
      await invalidate()
      toast.success('Connection created', {
        description: `${connection.name} is ready to launch.`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to create connection', {
        description: apiError.message,
      })
    },
  })

  const update = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: ConnectionUpdatePayload }) =>
      updateConnection(id, payload),
    onSuccess: async (connection) => {
      await invalidate()
      toast.success('Connection updated', {
        description: `${connection.name} was updated successfully.`,
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to update connection', {
        description: apiError.message,
      })
    },
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteConnection(id),
    onSuccess: async (_result, id) => {
      await invalidate()
      toast.success('Connection deleted', {
        description: 'The connection has been removed.',
      })
      queryClient.removeQueries({
        queryKey: [...CONNECTIONS_QUERY_BASE_KEY, { id }],
      })
    },
    onError: (error: unknown) => {
      const apiError = toApiError(error)
      toast.error('Failed to delete connection', {
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
