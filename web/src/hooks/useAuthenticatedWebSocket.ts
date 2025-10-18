import { useEffect, useMemo, useState } from 'react'
import { buildWebSocketUrl } from '@/lib/utils/websocket'
import { ensureFreshAccessToken } from '@/lib/api/client'
import { useAuth } from './useAuth'
import { useWebSocket, type UseWebSocketOptions, type UseWebSocketResult } from './useWebSocket'

interface UseAuthenticatedWebSocketOptions<TMessage> extends UseWebSocketOptions<TMessage> {
  params?: Record<string, string | number | boolean | undefined>
  enabled?: boolean
  tokenParamName?: string
  path?: string
}

export function useAuthenticatedWebSocket<TMessage>(
  options?: UseAuthenticatedWebSocketOptions<TMessage>
): UseWebSocketResult<TMessage> & {
  url: string
  ready: boolean
} {
  let path = '/ws'
  const { path: explicitPath, ...restOptions } = options || {}
  if (explicitPath) {
    path = explicitPath || '/ws'
  }

  const { params, enabled = true, tokenParamName = 'token', ...websocketOptions } = restOptions

  const { isAuthenticated, tokens } = useAuth({ autoInitialize: true })
  const [socketUrl, setSocketUrl] = useState('')
  const [ready, setReady] = useState(false)

  const paramsKey = useMemo(() => JSON.stringify(params ?? {}), [params])

  useEffect(() => {
    let cancelled = false

    const prepareUrl = async () => {
      if (!enabled || !isAuthenticated) {
        setSocketUrl('')
        setReady(true)
        return
      }

      const refreshed = await ensureFreshAccessToken()
      if (!refreshed?.accessToken) {
        setSocketUrl('')
        setReady(true)
        return
      }

      const query: Record<string, string> = { [tokenParamName]: refreshed.accessToken }
      if (params) {
        for (const [key, value] of Object.entries(params)) {
          if (value === undefined || value === null) {
            continue
          }
          query[key] = String(value)
        }
      }

      const url = buildWebSocketUrl(path, query)
      if (!cancelled) {
        setSocketUrl(url)
        setReady(true)
      }
    }

    void prepareUrl()

    return () => {
      cancelled = true
    }
  }, [
    enabled,
    isAuthenticated,
    params,
    paramsKey,
    path,
    tokenParamName,
    tokens?.accessToken,
    tokens?.expiresAt,
    tokens?.refreshToken,
  ])

  const websocket = useWebSocket<TMessage>(socketUrl, {
    ...websocketOptions,
    enabled: Boolean(socketUrl) && enabled,
  })

  return {
    ...websocket,
    url: socketUrl,
    ready,
  }
}
