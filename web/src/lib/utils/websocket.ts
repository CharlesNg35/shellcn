export function buildWebSocketUrl(path: string): string {
  if (typeof window === 'undefined') {
    return path
  }

  if (path.startsWith('ws://') || path.startsWith('wss://')) {
    return path
  }

  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host

  return `${protocol}//${host}${normalizedPath}`
}
