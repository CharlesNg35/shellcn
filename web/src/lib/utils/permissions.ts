import type {
  PermissionDefinition,
  PermissionIdentifier,
  PermissionRegistry,
} from '@/types/permission'

export function groupPermissionsByModule(
  registry: PermissionRegistry
): Record<string, PermissionDefinition[]> {
  return Object.values(registry).reduce<Record<string, PermissionDefinition[]>>((acc, perm) => {
    const moduleKey = perm.module || 'general'
    if (!acc[moduleKey]) {
      acc[moduleKey] = []
    }
    acc[moduleKey].push(perm)
    return acc
  }, {})
}

export function resolvePermissionDependencies(
  registry: PermissionRegistry,
  permissionId: PermissionIdentifier
): PermissionIdentifier[] {
  const visited = new Set<PermissionIdentifier>()
  const resolving = new Set<PermissionIdentifier>()

  const visit = (currentId: PermissionIdentifier) => {
    if (visited.has(currentId) || resolving.has(currentId)) {
      return
    }

    const definition = registry[currentId]
    if (!definition) {
      return
    }

    resolving.add(currentId)

    for (const dep of definition.depends_on ?? []) {
      if (!visited.has(dep)) {
        visit(dep)
        visited.add(dep)
      }
    }

    resolving.delete(currentId)
  }

  visit(permissionId)

  return Array.from(visited)
}

export function findPermissionDependents(
  registry: PermissionRegistry,
  permissionId: PermissionIdentifier
): PermissionIdentifier[] {
  const dependents = new Set<PermissionIdentifier>()

  const entries = Object.entries(registry)

  const visit = (currentId: PermissionIdentifier) => {
    for (const [candidateId, definition] of entries) {
      if (definition.depends_on?.includes(currentId) && !dependents.has(candidateId)) {
        dependents.add(candidateId)
        visit(candidateId as PermissionIdentifier)
      }
    }
  }

  visit(permissionId)

  return Array.from(dependents)
}
