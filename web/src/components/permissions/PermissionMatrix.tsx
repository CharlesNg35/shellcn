import { useEffect, useMemo, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, ChevronsDownUp, ChevronsUpDown, Search } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
import { humanizePermissionModule } from '@/lib/utils/permissionLabels'
import {
  groupPermissionsByModuleAndNamespace,
  type PermissionNamespace,
  type PermissionModuleGroup,
} from '@/lib/utils/permissions'
import type {
  PermissionDefinition,
  PermissionIdentifier,
  PermissionRegistry,
} from '@/types/permission'
import { Collapsible } from '@/components/ui/Collapsible'

interface PermissionMatrixProps {
  registry: PermissionRegistry | undefined
  loading?: boolean
  selected: ReadonlySet<PermissionIdentifier>
  onChange: (next: PermissionIdentifier[]) => void
  disabled?: boolean
}

function collectRequiredPermissions(
  registry: PermissionRegistry | undefined,
  permissionId: PermissionIdentifier,
  cache: Map<PermissionIdentifier, Set<PermissionIdentifier>>
): Set<PermissionIdentifier> {
  if (cache.has(permissionId)) {
    return cache.get(permissionId)!
  }

  const result = new Set<PermissionIdentifier>()
  const visit = (id: PermissionIdentifier) => {
    if (result.has(id)) {
      return
    }
    result.add(id)
    const definition = registry?.[id]
    if (!definition) {
      return
    }
    definition.depends_on?.forEach((dep) => visit(dep as PermissionIdentifier))
    definition.implies?.forEach((imp) => visit(imp as PermissionIdentifier))
  }

  visit(permissionId)
  cache.set(permissionId, result)
  return result
}

function deriveExplicitSelections(
  selected: ReadonlySet<PermissionIdentifier>,
  registry: PermissionRegistry | undefined
): Set<PermissionIdentifier> {
  const explicit = new Set<PermissionIdentifier>(selected)
  if (!registry) {
    return explicit
  }

  const cache = new Map<PermissionIdentifier, Set<PermissionIdentifier>>()
  const ids = Array.from(selected)

  for (const candidate of ids) {
    for (const other of ids) {
      if (candidate === other) {
        continue
      }
      const closure = collectRequiredPermissions(registry, other as PermissionIdentifier, cache)
      if (closure.has(candidate)) {
        explicit.delete(candidate)
        break
      }
    }
  }

  return explicit
}

function areSetsEqual<T>(a: Set<T>, b: Set<T>): boolean {
  if (a.size !== b.size) {
    return false
  }
  for (const value of a) {
    if (!b.has(value)) {
      return false
    }
  }
  return true
}

function getPermissionName(permission: PermissionDefinition): string {
  return permission.display_name?.trim() || permission.id
}

function sortPermissions(permissions: PermissionDefinition[]): PermissionDefinition[] {
  return [...permissions].sort((a, b) => getPermissionName(a).localeCompare(getPermissionName(b)))
}

function matchesSearch(permission: PermissionDefinition, search: string): boolean {
  if (!search) {
    return true
  }
  const haystack = `${permission.id} ${permission.display_name ?? ''} ${permission.description ?? ''} ${permission.module}`
  return haystack.toLowerCase().includes(search)
}

function countPermissionsInTree(tree: PermissionNamespace): number {
  let count = tree.permissions.length
  for (const child of tree.children.values()) {
    count += countPermissionsInTree(child)
  }
  return count
}

function filterPermissionTree(
  tree: PermissionNamespace,
  searchTerm: string
): PermissionNamespace | null {
  const filteredPermissions = tree.permissions.filter((p) => matchesSearch(p, searchTerm))
  const filteredChildren = new Map<string, PermissionNamespace>()

  for (const [key, child] of tree.children.entries()) {
    const filteredChild = filterPermissionTree(child, searchTerm)
    if (filteredChild) {
      filteredChildren.set(key, filteredChild)
    }
  }

  // Return null if no permissions match and no children match
  if (filteredPermissions.length === 0 && filteredChildren.size === 0) {
    return null
  }

  return {
    ...tree,
    permissions: filteredPermissions,
    children: filteredChildren,
  }
}

export function PermissionMatrix({
  registry,
  loading,
  selected,
  onChange,
  disabled,
}: PermissionMatrixProps) {
  const [explicitSelections, setExplicitSelections] = useState<Set<PermissionIdentifier>>(() =>
    deriveExplicitSelections(selected, registry)
  )
  const [searchTerm, setSearchTerm] = useState('')
  const [expandedModules, setExpandedModules] = useState<Set<string>>(() => new Set())
  const [expandedNamespaces, setExpandedNamespaces] = useState<Set<string>>(() => new Set())

  const normalisedSearch = searchTerm.trim().toLowerCase()

  useEffect(() => {
    const derived = deriveExplicitSelections(selected, registry)
    setExplicitSelections((prev) => (areSetsEqual(prev, derived) ? prev : derived))
  }, [selected, registry])

  const closureCacheRef = useRef<Map<PermissionIdentifier, Set<PermissionIdentifier>>>(new Map())

  useEffect(() => {
    closureCacheRef.current = new Map()
  }, [registry])

  const getClosure = (id: PermissionIdentifier) =>
    collectRequiredPermissions(registry, id, closureCacheRef.current)

  const computeFinalSelections = (
    explicitSet: Set<PermissionIdentifier>
  ): Set<PermissionIdentifier> => {
    if (!registry) {
      return new Set(explicitSet)
    }
    const final = new Set<PermissionIdentifier>()
    explicitSet.forEach((perm) => {
      getClosure(perm).forEach((id) => final.add(id))
    })
    return final
  }

  const moduleGroups = useMemo<PermissionModuleGroup[]>(() => {
    if (!registry) {
      return []
    }

    const groups = groupPermissionsByModuleAndNamespace(registry)

    // Apply search filter
    if (normalisedSearch) {
      return groups
        .map((group) => {
          const filteredTree = filterPermissionTree(group.namespaces, normalisedSearch)
          if (!filteredTree) {
            return null
          }
          return {
            ...group,
            namespaces: filteredTree,
          }
        })
        .filter((g): g is PermissionModuleGroup => g !== null)
    }

    return groups
  }, [registry, normalisedSearch])

  const impliedByIndex = useMemo(() => {
    if (!registry) {
      return {}
    }
    const map = Object.entries(registry).reduce<Record<string, PermissionIdentifier[]>>(
      (acc, [id, definition]) => {
        if (!definition.implies?.length) {
          return acc
        }
        definition.implies.forEach((implied) => {
          if (!acc[implied]) {
            acc[implied] = []
          }
          acc[implied].push(id as PermissionIdentifier)
        })
        return acc
      },
      {}
    )
    return map
  }, [registry])

  const totalPermissions = useMemo(() => {
    if (!registry) {
      return 0
    }
    return Object.keys(registry).length
  }, [registry])

  const getPermissionLabel = (permissionId: PermissionIdentifier): string => {
    if (!registry) {
      return permissionId
    }
    return registry[permissionId]?.display_name?.trim() || permissionId
  }

  const toggleModule = (moduleId: string) => {
    setExpandedModules((prev) => {
      const next = new Set(prev)
      if (next.has(moduleId)) {
        next.delete(moduleId)
      } else {
        next.add(moduleId)
      }
      return next
    })
  }

  const toggleNamespace = (fullPath: string) => {
    setExpandedNamespaces((prev) => {
      const next = new Set(prev)
      if (next.has(fullPath)) {
        next.delete(fullPath)
      } else {
        next.add(fullPath)
      }
      return next
    })
  }

  const expandAll = () => {
    const allModules = new Set(moduleGroups.map((g) => g.moduleId))
    const allNamespaces = new Set<string>()

    for (const group of moduleGroups) {
      const collectNamespaces = (tree: PermissionNamespace) => {
        if (tree.fullPath) {
          allNamespaces.add(tree.fullPath)
        }
        for (const child of tree.children.values()) {
          collectNamespaces(child)
        }
      }
      collectNamespaces(group.namespaces)
    }

    setExpandedModules(allModules)
    setExpandedNamespaces(allNamespaces)
  }

  const collapseAll = () => {
    setExpandedModules(new Set())
    setExpandedNamespaces(new Set())
  }

  const handleToggle = (permissionId: PermissionIdentifier, enable: boolean) => {
    const nextExplicit = new Set<PermissionIdentifier>(explicitSelections)

    if (enable) {
      nextExplicit.add(permissionId)
    } else {
      const removed = new Set<PermissionIdentifier>([permissionId])
      nextExplicit.delete(permissionId)

      if (registry) {
        let changed = true
        while (changed) {
          changed = false
          for (const explicit of Array.from(nextExplicit)) {
            const closure = getClosure(explicit)
            const intersects = Array.from(removed).some((removedId) => closure.has(removedId))
            if (intersects) {
              nextExplicit.delete(explicit)
              removed.add(explicit)
              changed = true
            }
          }
        }
      }
    }

    const finalSelections = computeFinalSelections(nextExplicit)
    setExplicitSelections(nextExplicit)
    onChange(Array.from(finalSelections))
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-full" />
        <div className="grid gap-4 md:grid-cols-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <div
              key={index}
              className="space-y-3 rounded-lg border border-border bg-card p-4 shadow-sm"
            >
              <Skeleton className="h-5 w-32" />
              {Array.from({ length: 4 }).map((__, inner) => (
                <div key={inner} className="flex items-center gap-3">
                  <Skeleton className="h-4 w-4" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="h-3 w-full" />
                  </div>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (!registry || totalPermissions === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border bg-muted/10 p-8 text-center text-sm text-muted-foreground">
        Permission registry is not available. Please try again later.
      </div>
    )
  }

  // Render helper for permissions
  const renderPermission = (permission: PermissionDefinition, depth: number) => {
    const isChecked = selected.has(permission.id)
    const impliedBySelected = (impliedByIndex[permission.id] ?? []).filter((id) => selected.has(id))
    const displayName = getPermissionLabel(permission.id)
    const checkboxId = `permission-${permission.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`
    const indentation = depth > 0 ? `${depth * 1.5}rem` : undefined

    const handleLabelClick = () => {
      if (disabled) {
        return
      }
      handleToggle(permission.id, !isChecked)
    }

    return (
      <div
        key={permission.id}
        className="py-1"
        style={indentation ? { marginLeft: indentation } : undefined}
      >
        <div
          className={cn(
            'flex flex-col gap-3 rounded-lg border border-border/70 bg-card/80 p-4 shadow-sm transition-colors duration-200',
            isChecked
              ? 'border-primary/70 bg-primary/5 shadow-inner'
              : 'hover:border-primary/40 hover:bg-muted/20'
          )}
        >
          <div className="flex items-center gap-3">
            <Checkbox
              id={checkboxId}
              checked={isChecked}
              onCheckedChange={(checked) => handleToggle(permission.id, Boolean(checked))}
              disabled={disabled}
              aria-label={displayName}
              aria-describedby={`${permission.id}-details`}
            />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <button
                  type="button"
                  onClick={handleLabelClick}
                  disabled={disabled}
                  className={cn(
                    'text-left transition-colors duration-150',
                    disabled ? 'cursor-not-allowed opacity-70' : 'hover:text-primary'
                  )}
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-semibold text-foreground">{displayName}</span>
                    <Badge
                      variant="outline"
                      className="text-[11px] font-normal uppercase tracking-wide text-muted-foreground"
                    >
                      {permission.id}
                    </Badge>
                  </div>
                </button>
              </div>

              {permission.description ? (
                <p className="text-sm text-muted-foreground">{permission.description}</p>
              ) : null}

              <div
                id={`${permission.id}-details`}
                className={cn(
                  'space-y-2 text-xs text-muted-foreground',
                  !(
                    impliedBySelected.length ||
                    (permission.implies && permission.implies.length > 0) ||
                    (permission.depends_on && permission.depends_on.length > 0)
                  ) && 'hidden'
                )}
              >
                {impliedBySelected.length ? (
                  <div className="flex flex-wrap items-center gap-1">
                    <span className="mr-1 font-medium uppercase tracking-wide">Implied by:</span>
                    {impliedBySelected.map((id) => (
                      <Badge
                        key={id}
                        variant="secondary"
                        className="text-[11px] uppercase tracking-wide"
                      >
                        {getPermissionLabel(id)}
                      </Badge>
                    ))}
                  </div>
                ) : permission.implies?.length ? (
                  <div className="flex flex-wrap items-center gap-1">
                    <span className="mr-1 font-medium uppercase tracking-wide">Grants:</span>
                    {permission.implies.map((dep) => (
                      <Badge
                        key={dep}
                        variant="outline"
                        className="text-[11px] uppercase tracking-wide"
                      >
                        {getPermissionLabel(dep)}
                      </Badge>
                    ))}
                  </div>
                ) : permission.depends_on?.length ? (
                  <div className="flex flex-wrap items-center gap-1">
                    <span className="mr-1 font-medium uppercase tracking-wide">Depends on:</span>
                    {permission.depends_on.map((dep) => (
                      <Badge
                        key={dep}
                        variant={selected.has(dep) ? 'outline' : 'secondary'}
                        className={cn(
                          'text-[11px] uppercase tracking-wide',
                          selected.has(dep) ? 'border-primary text-primary' : ''
                        )}
                      >
                        {getPermissionLabel(dep)}
                      </Badge>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Render helper for namespace nodes
  const renderNamespace = (namespace: PermissionNamespace, depth: number): React.JSX.Element => {
    if (!namespace.name) {
      // Root node - render children only
      const sortedChildren = Array.from(namespace.children.values()).sort((a, b) =>
        a.name.localeCompare(b.name)
      )
      return (
        <>
          {sortPermissions(namespace.permissions).map((p) => renderPermission(p, depth))}
          {sortedChildren.map((child) => renderNamespace(child, depth))}
        </>
      )
    }

    const isExpanded = expandedNamespaces.has(namespace.fullPath)
    const hasChildren = namespace.children.size > 0
    const permissionCount = countPermissionsInTree(namespace)

    const indentation = `${depth * 1.5 + 1.25}rem`

    return (
      <div key={namespace.fullPath} className="border-t border-border/60">
        {/* Namespace header */}
        <button
          onClick={() => toggleNamespace(namespace.fullPath)}
          className="flex w-full items-center gap-2 px-5 py-2.5 text-left hover:bg-muted/50"
          style={{ paddingLeft: indentation }}
          aria-expanded={isExpanded}
        >
          {hasChildren || namespace.permissions.length > 0 ? (
            <span className="inline-flex h-4 w-4 items-center justify-center text-muted-foreground transition-transform duration-200">
              {isExpanded ? (
                <ChevronDown className="h-4 w-4" />
              ) : (
                <ChevronRight className="h-4 w-4" />
              )}
            </span>
          ) : (
            <div className="h-4 w-4 shrink-0" />
          )}
          <span className="text-sm font-semibold text-foreground capitalize">{namespace.name}</span>
          <Badge variant="secondary" className="ml-auto text-xs">
            {permissionCount}
          </Badge>
        </button>

        <Collapsible isOpen={isExpanded}>
          <div className="space-y-1 pb-2">
            {sortPermissions(namespace.permissions).map((p) => renderPermission(p, depth + 1))}
            {Array.from(namespace.children.values())
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((child) => renderNamespace(child, depth + 1))}
          </div>
        </Collapsible>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <Badge variant="outline" className="font-medium">
            {selected.size} selected
          </Badge>
          <span>out of {totalPermissions} permissions</span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" variant="outline" size="sm" onClick={expandAll} className="gap-1.5">
            <ChevronsDownUp className="h-4 w-4" />
            Expand All
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={collapseAll}
            className="gap-1.5"
          >
            <ChevronsUpDown className="h-4 w-4" />
            Collapse All
          </Button>
          <div className="relative w-full max-w-xs sm:max-w-sm">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              placeholder="Search permissions..."
              className="pl-9"
              aria-label="Search permissions"
            />
          </div>
        </div>
      </div>

      <div className="space-y-4">
        {moduleGroups.map((group) => {
          const isModuleExpanded = expandedModules.has(group.moduleId)
          const permissionCount = countPermissionsInTree(group.namespaces)

          return (
            <div
              key={group.moduleId}
              className="rounded-lg border border-border bg-card shadow-sm transition-colors"
            >
              {/* Module header */}
              <button
                onClick={() => toggleModule(group.moduleId)}
                className="flex w-full items-center justify-between border-b border-border/80 px-5 py-3 text-left hover:bg-muted/30"
                aria-expanded={isModuleExpanded}
              >
                <div className="flex items-center gap-3">
                  <span className="inline-flex h-5 w-5 items-center justify-center text-muted-foreground transition-transform duration-200">
                    {isModuleExpanded ? (
                      <ChevronDown className="h-5 w-5" />
                    ) : (
                      <ChevronRight className="h-5 w-5" />
                    )}
                  </span>
                  <div>
                    <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                      {humanizePermissionModule(group.moduleId)}
                    </h3>
                    <p className="text-xs text-muted-foreground/80">
                      {permissionCount} permission{permissionCount === 1 ? '' : 's'}
                    </p>
                  </div>
                </div>
              </button>

              {/* Module content */}
              <Collapsible isOpen={isModuleExpanded}>
                <div className="divide-y divide-border/70">
                  {renderNamespace(group.namespaces, 0)}
                </div>
              </Collapsible>
            </div>
          )
        })}

        {moduleGroups.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border bg-muted/10 p-8 text-center text-sm text-muted-foreground">
            No permissions match your search.
          </div>
        ) : null}
      </div>
    </div>
  )
}
