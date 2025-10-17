import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'

import { launchActiveSession } from '@/lib/api/active-sessions'
import { fetchConnectionById } from '@/lib/api/connections'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import type { ConnectionRecord } from '@/types/connections'
import type { WorkspaceDescriptor } from '@/workspaces/types'
import {
  FALLBACK_DESCRIPTOR,
  getWorkspaceDescriptor,
  getWorkspaceDescriptorForProtocol,
} from '@/workspaces/protocolWorkspaceRegistry'
import { useConnectionTemplate } from '@/hooks/useConnectionTemplate'
import { useActiveConnections, ACTIVE_CONNECTIONS_QUERY_KEY } from '@/hooks/useActiveConnections'
import { CONNECTIONS_QUERY_BASE_KEY } from '@/hooks/useConnections'
import type { ActiveConnectionSession } from '@/types/connections'

export interface LaunchDialogState {
  isOpen: boolean
  connection: ConnectionRecord | null
  descriptor: WorkspaceDescriptor
}

export interface LaunchSessionOptions {
  fieldsOverride?: Record<string, unknown>
}

export function useLaunchConnection() {
  const [state, setState] = useState<LaunchDialogState>({
    isOpen: false,
    connection: null,
    descriptor: FALLBACK_DESCRIPTOR,
  })
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const open = useCallback((connection: ConnectionRecord) => {
    setState({
      isOpen: true,
      connection,
      descriptor: getWorkspaceDescriptorForProtocol(connection.protocol_id),
    })
    setErrorMessage(null)
  }, [])

  const openById = useCallback(
    async (connectionId: string) => {
      try {
        const connection = await queryClient.fetchQuery({
          queryKey: ['connections', 'detail', connectionId],
          queryFn: () => fetchConnectionById(connectionId),
          staleTime: 60_000,
        })
        open(connection)
      } catch (error) {
        const apiError = toApiError(error)
        toast.error('Unable to load connection', {
          description: apiError.message,
        })
      }
    },
    [open, queryClient]
  )

  const close = useCallback(() => {
    setState((prev) => ({
      ...prev,
      isOpen: false,
      connection: null,
      descriptor: FALLBACK_DESCRIPTOR,
    }))
    setErrorMessage(null)
  }, [])

  const connection = state.connection

  const templateQuery = useConnectionTemplate(connection?.protocol_id)

  const activeSessionsQuery = useActiveConnections({
    protocol_id: connection?.protocol_id,
    enabled: state.isOpen && Boolean(connection),
    refetchInterval: state.isOpen ? 10_000 : false,
  })

  const matchingSessions = useMemo<ActiveConnectionSession[]>(() => {
    if (!connection) {
      return []
    }
    const records = activeSessionsQuery.data ?? []
    return records.filter((session) => session.connection_id === connection.id)
  }, [activeSessionsQuery.data, connection])

  const descriptor = state.descriptor ?? FALLBACK_DESCRIPTOR

  const mutation = useMutation({
    mutationFn: async (options: LaunchSessionOptions = {}) => {
      if (!connection) {
        throw new Error('No connection selected')
      }
      return launchActiveSession({
        connection_id: connection.id,
        protocol_id: connection.protocol_id,
        fields_override: options.fieldsOverride,
      })
    },
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ACTIVE_CONNECTIONS_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: CONNECTIONS_QUERY_BASE_KEY, exact: false })
      close()
      const session = response.session
      const descriptorFromResponse =
        getWorkspaceDescriptor(response.descriptor?.id ?? session.descriptor_id) ??
        getWorkspaceDescriptorForProtocol(session.protocol_id)
      const targetDescriptor = descriptorFromResponse ?? descriptor
      const route = response.descriptor?.default_route ?? targetDescriptor.defaultRoute(session.id)
      toast.success('Session launched', {
        description: `${connection?.name ?? 'Connection'} is ready`,
      })
      navigate(route)
    },
    onError: (error) => {
      const apiError = toApiError(error)
      setErrorMessage(apiError.message)
      toast.error('Unable to launch connection', {
        description: apiError.message,
      })
    },
  })

  const { mutateAsync, isPending } = mutation

  const launch = useCallback(
    (options?: LaunchSessionOptions) => mutateAsync(options ?? {}),
    [mutateAsync]
  )

  const resumeSession = useCallback(
    (session: ActiveConnectionSession) => {
      const descriptorForSession =
        getWorkspaceDescriptor(session.descriptor_id) ??
        getWorkspaceDescriptorForProtocol(session.protocol_id)
      close()
      navigate(descriptorForSession.defaultRoute(session.id))
    },
    [close, navigate]
  )

  return {
    state,
    open,
    openById,
    close,
    descriptor,
    activeSessions: matchingSessions,
    isFetchingSessions: activeSessionsQuery.isLoading,
    template: templateQuery.data ?? null,
    launch,
    isLaunching: isPending,
    errorMessage,
    resumeSession,
  }
}
