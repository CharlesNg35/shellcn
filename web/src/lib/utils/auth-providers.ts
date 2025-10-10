export function serializeAttributeMapping(mapping?: Record<string, string>): string {
  if (!mapping) {
    return ''
  }
  return Object.entries(mapping)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

export function parseAttributeMapping(text: string): Record<string, string> {
  const result: Record<string, string> = {}
  const lines = text.split(/\r?\n/)
  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) {
      continue
    }
    const separatorIndex = line.indexOf('=')
    if (separatorIndex === -1) {
      throw new Error('Each attribute mapping line must follow the format key=value')
    }
    const key = line.slice(0, separatorIndex).trim()
    const value = line.slice(separatorIndex + 1).trim()
    if (!key || !value) {
      throw new Error('Attribute mapping entries require both a key and a value')
    }
    result[key] = value
  }
  return result
}
