import type { ActiveSessionParticipant } from '@/types/connections'

export function normalizePath(path?: string): string {
  const trimmed = path?.trim()
  if (!trimmed) {
    return ''
  }
  // Handle absolute paths - keep them as-is after trimming trailing slashes
  if (trimmed.startsWith('/')) {
    const cleaned = trimmed.replace(/\/+$/, '')
    return cleaned || '/'
  }
  // Handle relative paths - convert to empty string for default
  if (trimmed === '.' || trimmed === '/') {
    return ''
  }
  return trimmed.replace(/\/+$/, '')
}

export function displayPath(path: string): string {
  const trimmed = path?.trim()
  if (!trimmed) {
    return '/'
  }
  // Already absolute
  if (trimmed.startsWith('/')) {
    return trimmed
  }
  // Make absolute
  return `/${trimmed}`
}

export function resolveChildPath(basePath: string, name: string): string {
  const safeName = name.replace(/^\/+/, '')
  const base = basePath.trim()

  // If base is empty, root, or dot notation, return name with leading slash
  if (!base || base === '/' || base === '.') {
    return `/${safeName}`
  }

  // If base is absolute, append name
  if (base.startsWith('/')) {
    return `${base.replace(/\/+$/, '')}/${safeName}`
  }

  // Fallback: treat as relative (prepend slash)
  return `/${base.replace(/\/+$/, '')}/${safeName}`
}

export function parentPath(path: string): string {
  const trimmed = path?.trim()
  if (!trimmed || trimmed === '/') {
    return '/'
  }

  const normalized = trimmed.replace(/\/+$/, '')
  const slashIndex = normalized.lastIndexOf('/')

  // If no slash or only leading slash, we're at root
  if (slashIndex <= 0) {
    return '/'
  }

  return normalized.slice(0, slashIndex)
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value === undefined || value === null) {
    return '—'
  }
  if (value === 0) {
    return '0 B'
  }
  const absValue = Math.abs(value)
  if (absValue < 1024) {
    return `${value} B`
  }
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
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
