import type {
  PermissionDefinition,
  PermissionIdentifier,
  PermissionRegistry,
} from '@/types/permission'

export interface PermissionNamespace {
  name: string
  fullPath: string
  permissions: PermissionDefinition[]
  children: Map<string, PermissionNamespace>
}

export interface PermissionModuleGroup {
  moduleId: string
  namespaces: PermissionNamespace
}

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

/**
 * Extracts namespace from permission ID
 * e.g., "k8s.pod.view" -> ["k8s", "pod"]
 *       "user.view" -> ["user"]
 */
export function extractNamespace(permissionId: string): string[] {
  const parts = permissionId.split('.')
  // Return all parts except the last one (action)
  return parts.slice(0, -1)
}

/**
 * Builds a hierarchical tree of permissions organized by namespace
 */
export function buildPermissionTree(permissions: PermissionDefinition[]): PermissionNamespace {
  const root: PermissionNamespace = {
    name: '',
    fullPath: '',
    permissions: [],
    children: new Map(),
  }

  for (const permission of permissions) {
    const namespaceParts = extractNamespace(permission.id)

    if (namespaceParts.length === 0) {
      // Permission without namespace goes to root
      root.permissions.push(permission)
      continue
    }

    let current = root
    let path = ''

    for (let i = 0; i < namespaceParts.length; i++) {
      const part = namespaceParts[i]
      path = path ? `${path}.${part}` : part

      if (!current.children.has(part)) {
        current.children.set(part, {
          name: part,
          fullPath: path,
          permissions: [],
          children: new Map(),
        })
      }

      current = current.children.get(part)!

      // If this is the last namespace part, add the permission here
      if (i === namespaceParts.length - 1) {
        current.permissions.push(permission)
      }
    }
  }

  return root
}

/**
 * Groups permissions by module, then organizes each module's permissions into a hierarchical tree
 */
export function groupPermissionsByModuleAndNamespace(
  registry: PermissionRegistry
): PermissionModuleGroup[] {
  const moduleGroups = groupPermissionsByModule(registry)

  return Object.entries(moduleGroups)
    .map(([moduleId, permissions]) => ({
      moduleId,
      namespaces: buildPermissionTree(permissions),
    }))
    .sort((a, b) => a.moduleId.localeCompare(b.moduleId))
}

/**
 * Flattens a permission tree into a list for rendering
 * Returns a tuple of [namespace, depth, permissions]
 */
export function flattenPermissionTree(
  tree: PermissionNamespace,
  depth = 0
): Array<{ namespace: PermissionNamespace; depth: number }> {
  const result: Array<{ namespace: PermissionNamespace; depth: number }> = []

  // Add current node if it has permissions or children
  if (tree.permissions.length > 0 || tree.children.size > 0) {
    result.push({ namespace: tree, depth })
  }

  // Recursively add children
  const sortedChildren = Array.from(tree.children.values()).sort((a, b) =>
    a.name.localeCompare(b.name)
  )

  for (const child of sortedChildren) {
    result.push(...flattenPermissionTree(child, depth + 1))
  }

  return result
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
