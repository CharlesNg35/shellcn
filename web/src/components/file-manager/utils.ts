import type { ActiveSessionParticipant } from '@/types/connections'

export function normalizePath(path?: string): string {
  const trimmed = path?.trim()
  if (!trimmed || trimmed === '.' || trimmed === '/') {
    return '.'
  }
  return trimmed.replace(/^\/+/, '').replace(/\/+$/, '')
}

export function displayPath(path: string): string {
  if (!path || path === '.' || path === '/') {
    return '/'
  }
  return path.startsWith('/') ? path : `/${path}`
}

export function resolveChildPath(basePath: string, name: string): string {
  const safeName = name.replace(/^\//, '')
  if (!basePath || basePath === '.' || basePath === '/') {
    return safeName
  }
  return `${basePath.replace(/\/+$/, '')}/${safeName}`
}

export function parentPath(path: string): string {
  if (!path || path === '.' || path === '/') {
    return '.'
  }
  const normalized = path.replace(/\/+$/, '')
  const slashIndex = normalized.lastIndexOf('/')
  if (slashIndex <= 0) {
    return '.'
  }
  return normalized.slice(0, slashIndex)
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  const absValue = Math.abs(value)
  if (absValue < 1024) {
    return `${value} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let index = -1
  let size = absValue
  do {
    size /= 1024
    index += 1
  } while (size >= 1024 && index < units.length - 1)
  const formatted = `${value < 0 ? '-' : ''}${size.toFixed(size >= 10 ? 0 : 1)} ${units[index]}`
  return formatted
}

export function extractNameFromPath(path: string): string {
  if (!path) {
    return ''
  }
  const cleaned = path.replace(/\/+$/, '')
  const segments = cleaned.split('/')
  return segments[segments.length - 1] || cleaned
}

export function resolveParticipantName(
  participants: Record<string, ActiveSessionParticipant> | undefined,
  userId: string | undefined,
  fallback?: string
): string | undefined {
  if (!userId) {
    return fallback
  }
  const participant = participants?.[userId]
  return participant?.user_name || fallback
}

export function formatLabel(value?: string) {
  if (!value) {
    return ''
  }
  return value.charAt(0).toUpperCase() + value.slice(1)
}

export function sortEntries<T extends { isDir: boolean; name: string }>(entries: T[]): T[] {
  return [...entries].sort((a, b) => {
    if (a.isDir && !b.isDir) {
      return -1
    }
    if (!a.isDir && b.isDir) {
      return 1
    }
    return a.name.localeCompare(b.name)
  })
}
