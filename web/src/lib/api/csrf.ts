import type { InternalAxiosRequestConfig } from 'axios'

const CSRF_COOKIE_NAME = 'shellcn_csrf'
const CSRF_HEADER_NAME = 'X-CSRF-Token'
const SAFE_METHODS = new Set(['get', 'head', 'options', 'trace'])

function getCookie(name: string): string | undefined {
  if (typeof document === 'undefined') {
    return undefined
  }

  const cookies = document.cookie?.split(';') ?? []
  for (const cookie of cookies) {
    const [key, ...rest] = cookie.trim().split('=')
    if (key === name) {
      return decodeURIComponent(rest.join('='))
    }
  }
  return undefined
}

export function attachCSRFToken(config: InternalAxiosRequestConfig) {
  if (typeof window === 'undefined') {
    return
  }

  const token = getCookie(CSRF_COOKIE_NAME)
  if (!token) {
    return
  }

  const method = (config.method ?? 'get').toLowerCase()
  if (!SAFE_METHODS.has(method) || config.headers?.[CSRF_HEADER_NAME] == null) {
    config.headers = config.headers ?? {}
    config.headers[CSRF_HEADER_NAME] = token
  }

  if (config.withCredentials === undefined) {
    config.withCredentials = true
  }
}
