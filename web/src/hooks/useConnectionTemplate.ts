import { useQuery } from '@tanstack/react-query'
import { fetchConnectionTemplate } from '@/lib/api/protocols'
import type { ConnectionTemplate } from '@/types/protocols'

export function useConnectionTemplate(protocolId?: string | null) {
  return useQuery<ConnectionTemplate | null>({
    queryKey: ['protocols', protocolId, 'connection-template'],
    queryFn: () => {
      if (!protocolId) {
        return Promise.resolve(null)
      }
      return fetchConnectionTemplate(protocolId)
    },
    enabled: Boolean(protocolId),
    staleTime: 60_000,
  })
}
