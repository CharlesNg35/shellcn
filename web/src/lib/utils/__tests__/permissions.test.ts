import { describe, expect, it } from 'vitest'
import type { PermissionRegistry } from '@/types/permission'
import { findPermissionDependents, resolvePermissionDependencies } from '@/lib/utils/permissions'

const registry: PermissionRegistry = {
  'user.view': {
    id: 'user.view',
    module: 'core',
    description: 'View users',
    depends_on: [],
    implies: [],
  },
  'user.update': {
    id: 'user.update',
    module: 'core',
    description: 'Update users',
    depends_on: ['user.view'],
    implies: [],
  },
  'user.delete': {
    id: 'user.delete',
    module: 'core',
    description: 'Delete users',
    depends_on: ['user.view', 'user.update'],
    implies: [],
  },
}

describe('permission utilities', () => {
  it('resolves dependency chain for a permission', () => {
    const dependencies = resolvePermissionDependencies(registry, 'user.delete')
    expect(dependencies).toEqual(['user.view', 'user.update'])
  })

  it('finds dependents recursively', () => {
    const dependents = findPermissionDependents(registry, 'user.view')
    expect(dependents).toEqual(['user.update', 'user.delete'])
  })
})
