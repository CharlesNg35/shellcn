import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { SSHProtocolSettings } from '@/types/protocol-settings'
import type { ApiError } from '@/lib/api/http'
import { fetchSSHProtocolSettings, updateSSHProtocolSettings } from '@/lib/api/protocol-settings'

const QUERY_KEY = ['protocol-settings', 'ssh']

export function useSSHProtocolSettings() {
  const queryClient = useQueryClient()

  const query = useQuery<SSHProtocolSettings, ApiError>({
    queryKey: QUERY_KEY,
    queryFn: fetchSSHProtocolSettings,
  })

  const mutation = useMutation<SSHProtocolSettings, ApiError, SSHProtocolSettings>({
    mutationFn: updateSSHProtocolSettings,
    onSuccess: (data) => {
      queryClient.setQueryData(QUERY_KEY, data)
    },
  })

  return {
    ...query,
    update: mutation,
  }
}
