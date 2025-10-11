type QueryValue = string | number | boolean | undefined | null

export function buildWebSocketUrl(path: string, params?: Record<string, QueryValue>): string {
  if (typeof window === 'undefined') {
    return path
  }

  const appendParams = (url: URL) => {
    if (!params) {
      return
    }
    Object.entries(params).forEach(([key, value]) => {
      if (value === undefined || value === null) {
        return
      }
      url.searchParams.set(key, String(value))
    })
  }

  if (path.startsWith('ws://') || path.startsWith('wss://')) {
    const url = new URL(path)
    appendParams(url)
    return url.toString()
  }

  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const base = `${protocol}//${host}`
  const url = new URL(normalizedPath, base)

  appendParams(url)

  return url.toString()
}
