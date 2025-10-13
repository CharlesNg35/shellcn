import axios, { AxiosError, type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import type { AuthTokens, RefreshResponsePayload } from '@/types/auth'
import type { ApiResponse } from '@/types/api'
import { clearTokens, getTokens, setTokens } from './token-storage'
import { attachCSRFToken } from './csrf'
import { toApiError, unwrapResponse } from './http'
import { toAuthTokens } from './transformers'

type RetryableRequestConfig = InternalAxiosRequestConfig & {
  _retry?: boolean
}

const AUTH_REFRESH_ENDPOINT = '/auth/refresh'
const AUTH_LOGIN_ENDPOINT = '/auth/login'

const apiClient: AxiosInstance = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

const refreshClient: AxiosInstance = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

let refreshPromise: Promise<AuthTokens | null> | null = null

apiClient.interceptors.request.use((config) => {
  const tokens = getTokens()

  if (tokens?.accessToken) {
    config.headers = config.headers ?? {}
    config.headers.Authorization = `Bearer ${tokens.accessToken}`
  }

  attachCSRFToken(config)

  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const { response, config } = error

    if (!response || !config) {
      return Promise.reject(toApiError(error))
    }

    const status = response.status

    if (status !== 401) {
      return Promise.reject(toApiError(error))
    }

    const requestConfig = config as RetryableRequestConfig

    if (requestConfig._retry) {
      return Promise.reject(toApiError(error))
    }

    const requestUrl = requestConfig.url ?? ''

    if (
      requestUrl.startsWith(AUTH_REFRESH_ENDPOINT) ||
      requestUrl.startsWith(AUTH_LOGIN_ENDPOINT)
    ) {
      return Promise.reject(toApiError(error))
    }

    const tokens = getTokens()

    if (!tokens?.refreshToken) {
      clearTokens()
      return Promise.reject(toApiError(error))
    }

    try {
      const refreshed = await enqueueTokenRefresh(tokens.refreshToken)

      if (!refreshed?.accessToken) {
        throw error
      }

      requestConfig._retry = true
      requestConfig.headers = requestConfig.headers ?? {}
      requestConfig.headers.Authorization = `Bearer ${refreshed.accessToken}`

      return apiClient(requestConfig)
    } catch (refreshError) {
      clearTokens()
      return Promise.reject(toApiError(refreshError))
    }
  }
)

async function enqueueTokenRefresh(refreshToken: string): Promise<AuthTokens | null> {
  if (!refreshPromise) {
    refreshPromise = refreshAccessToken(refreshToken).finally(() => {
      refreshPromise = null
    })
  }

  return refreshPromise
}

async function refreshAccessToken(refreshToken: string): Promise<AuthTokens | null> {
  try {
    const response = await refreshClient.post<ApiResponse<RefreshResponsePayload>>(
      AUTH_REFRESH_ENDPOINT,
      {
        refresh_token: refreshToken,
      }
    )

    const data = unwrapResponse(response)
    const tokens = toAuthTokens(data)

    if (tokens) {
      setTokens(tokens)
    } else {
      clearTokens()
    }

    return tokens
  } catch {
    clearTokens()
    return null
  }
}

export { apiClient }
