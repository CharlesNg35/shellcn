import type {
  CredentialField,
  CredentialFieldComparable,
  CredentialFieldMetadata,
  CredentialFieldVisibilityRule,
} from '@/types/vault'

type PayloadValues = Record<string, unknown>

export function isCredentialFieldVisible(
  field: CredentialField,
  payload: PayloadValues | undefined
): boolean {
  const metadata = (field.metadata ?? {}) as CredentialFieldMetadata
  if (!metadata.visibility) {
    return true
  }

  const rules = Array.isArray(metadata.visibility) ? metadata.visibility : [metadata.visibility]

  if (rules.length === 0) {
    return true
  }

  const values = payload ?? {}
  const results = rules.map((rule) => evaluateRule(rule, values))
  const overallMode = metadata.visibility_mode === 'any' ? 'any' : 'all'

  return overallMode === 'any' ? results.some(Boolean) : results.every(Boolean)
}

function evaluateRule(rule: CredentialFieldVisibilityRule, payload: PayloadValues): boolean {
  const value = getFieldValue(payload, rule.field)

  if (rule.exists && !hasValue(value)) {
    return false
  }

  if (rule.not_exists && hasValue(value)) {
    return false
  }

  if (rule.truthy && !isTruthy(value)) {
    return false
  }

  if (rule.falsy && isTruthy(value)) {
    return false
  }

  if (rule.equals !== undefined) {
    const targets = normalizeComparables(rule.equals)
    if (!targets.some((target) => isEqual(value, target))) {
      return false
    }
  }

  if (rule.in) {
    const targets = normalizeComparables(rule.in)
    if (!targets.some((target) => isEqual(value, target))) {
      return false
    }
  }

  if (rule.not_equals !== undefined) {
    const targets = normalizeComparables(rule.not_equals)
    if (targets.some((target) => isEqual(value, target))) {
      return false
    }
  }

  if (rule.not_in) {
    const targets = normalizeComparables(rule.not_in)
    if (targets.some((target) => isEqual(value, target))) {
      return false
    }
  }

  return true
}

function normalizeComparables(
  value: CredentialFieldComparable | CredentialFieldComparable[]
): CredentialFieldComparable[] {
  return Array.isArray(value) ? value : [value]
}

function hasValue(value: unknown): boolean {
  if (Array.isArray(value)) {
    return value.length > 0
  }
  return value !== null && value !== undefined && value !== ''
}

function isTruthy(value: unknown): boolean {
  if (!hasValue(value)) {
    return false
  }
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'number') {
    return value !== 0
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['false', '0', 'no', 'off'].includes(normalized)) {
      return false
    }
  }
  return true
}

function isEqual(value: unknown, target: CredentialFieldComparable): boolean {
  if (typeof target === 'boolean') {
    return Boolean(value) === target
  }
  if (typeof target === 'number') {
    const numericValue = typeof value === 'number' ? value : Number(value)
    return !Number.isNaN(numericValue) && numericValue === target
  }
  return String(value ?? '').trim() === String(target ?? '').trim()
}

function getFieldValue(payload: PayloadValues, fieldPath: string): unknown {
  if (!fieldPath) {
    return undefined
  }

  const segments = fieldPath.split('.').map((segment) => segment.trim())
  let current: unknown = payload

  for (const segment of segments) {
    if (!segment) {
      return undefined
    }
    if (current && typeof current === 'object' && !Array.isArray(current)) {
      current = (current as Record<string, unknown>)[segment]
    } else {
      return undefined
    }
  }

  return current
}
