import type { ConnectionTemplateField } from '@/types/protocols'

export type TemplateValueMap = Record<string, unknown>

export function isFieldVisible(field: ConnectionTemplateField, values: TemplateValueMap): boolean {
  if (!field.dependencies?.length) {
    return true
  }
  return field.dependencies.every((dependency) => dependencySatisfied(dependency, values))
}

function dependencySatisfied(
  dependency: NonNullable<ConnectionTemplateField['dependencies']>[number],
  values: TemplateValueMap
): boolean {
  const actual = values?.[dependency.field]
  const expected = dependency.equals
  if (Array.isArray(expected)) {
    return expected.some((option) => compareTemplateValues(actual, option))
  }
  return compareTemplateValues(actual, expected)
}

function compareTemplateValues(actual: unknown, expected: unknown): boolean {
  if (expected === null || expected === undefined) {
    return actual === null || actual === undefined
  }
  if (typeof expected === 'boolean') {
    return Boolean(actual) === expected
  }
  if (typeof expected === 'number') {
    const numericActual =
      typeof actual === 'number'
        ? actual
        : typeof actual === 'string'
          ? Number.parseFloat(actual)
          : Number(actual)
    return Number.isFinite(numericActual) && numericActual === expected
  }
  if (typeof expected === 'string') {
    return String(actual ?? '').trim() === expected.trim()
  }
  return actual === expected
}
