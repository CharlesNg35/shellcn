import { useMemo, useState } from 'react'
import {
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  ChevronsUpDown,
  Search,
  ShieldAlert,
} from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
import { humanizePermissionModule } from '@/lib/utils/permissionLabels'
import {
  findPermissionDependents,
  groupPermissionsByModuleAndNamespace,
  resolvePermissionDependencies,
  type PermissionNamespace,
  type PermissionModuleGroup,
} from '@/lib/utils/permissions'
import type {
  PermissionDefinition,
  PermissionIdentifier,
  PermissionRegistry,
} from '@/types/permission'

interface PermissionMatrixProps {
  registry: PermissionRegistry | undefined
  loading?: boolean
  selected: ReadonlySet<PermissionIdentifier>
  onChange: (next: PermissionIdentifier[]) => void
  disabled?: boolean
}

function sortPermissions(permissions: PermissionDefinition[]): PermissionDefinition[] {
  return [...permissions].sort((a, b) => a.id.localeCompare(b.id))
}

function matchesSearch(permission: PermissionDefinition, search: string): boolean {
  if (!search) {
    return true
  }
  const haystack = `${permission.id} ${permission.description ?? ''} ${permission.module}`
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
  const [searchTerm, setSearchTerm] = useState('')
  const [expandedModules, setExpandedModules] = useState<Set<string>>(() => new Set())
  const [expandedNamespaces, setExpandedNamespaces] = useState<Set<string>>(() => new Set())

  const normalisedSearch = searchTerm.trim().toLowerCase()

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

  const dependentIndex = useMemo(() => {
    if (!registry) {
      return {}
    }
    return Object.keys(registry).reduce<Record<string, PermissionIdentifier[]>>((acc, id) => {
      acc[id] = findPermissionDependents(registry, id)
      return acc
    }, {})
  }, [registry])

  const totalPermissions = useMemo(() => {
    if (!registry) {
      return 0
    }
    return Object.keys(registry).length
  }, [registry])

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
    if (!registry) {
      return
    }

    const next = new Set<PermissionIdentifier>(selected)

    if (enable) {
      next.add(permissionId)
      const required = resolvePermissionDependencies(registry, permissionId)
      required.forEach((dep) => next.add(dep))
    } else {
      next.delete(permissionId)
      const dependents = findPermissionDependents(registry, permissionId)
      dependents.forEach((dep) => next.delete(dep))
    }

    onChange(Array.from(next))
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
    const dependents = dependentIndex[permission.id] ?? []
    const blockingDependents = dependents.filter((dep) => selected.has(dep))
    const isLocked = !disabled && isChecked && blockingDependents.length > 0

    return (
      <div
        key={permission.id}
        className={cn(
          'flex flex-col gap-2 px-5 py-3 md:flex-row md:items-start md:justify-between',
          isChecked ? 'bg-primary/5' : 'hover:bg-muted/40'
        )}
        style={{ paddingLeft: `${depth * 1.5 + 1.25}rem` }}
      >
        <div className="flex flex-1 gap-3">
          <div className="pt-0.5">
            <Checkbox
              checked={isChecked}
              onCheckedChange={(checked) => handleToggle(permission.id, Boolean(checked))}
              disabled={disabled || (isChecked && isLocked)}
              aria-label={permission.id}
              aria-describedby={`${permission.id}-details`}
            />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="font-medium text-foreground">{permission.id}</span>
              {isLocked ? (
                <Badge variant="outline" className="flex items-center gap-1 text-xs">
                  <ShieldAlert className="h-3 w-3" />
                  Required by {blockingDependents.length}
                </Badge>
              ) : null}
            </div>
            {permission.description ? (
              <p className="mt-1 text-sm text-muted-foreground">{permission.description}</p>
            ) : null}
            <div className="mt-2 space-y-2 text-xs text-muted-foreground">
              {permission.depends_on?.length ? (
                <div className="flex flex-wrap items-center gap-1" id={`${permission.id}-details`}>
                  <span className="mr-1 font-medium">Depends on:</span>
                  {permission.depends_on.map((dep) => (
                    <Badge
                      key={dep}
                      variant={selected.has(dep) ? 'outline' : 'secondary'}
                      className={cn(
                        'text-[11px] uppercase tracking-wide',
                        selected.has(dep) ? 'border-primary text-primary' : ''
                      )}
                    >
                      {dep}
                    </Badge>
                  ))}
                </div>
              ) : null}
              {permission.implies?.length ? (
                <div className="flex flex-wrap items-center gap-1">
                  <span className="mr-1 font-medium">Implies:</span>
                  {permission.implies.map((dep) => (
                    <Badge
                      key={dep}
                      variant="outline"
                      className="text-[11px] uppercase tracking-wide"
                    >
                      {dep}
                    </Badge>
                  ))}
                </div>
              ) : null}
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

    return (
      <div key={namespace.fullPath}>
        {/* Namespace header */}
        <button
          onClick={() => toggleNamespace(namespace.fullPath)}
          className="flex w-full items-center gap-2 border-t border-border/50 bg-muted/30 px-5 py-2.5 text-left hover:bg-muted/50"
          style={{ paddingLeft: `${depth * 1.5 + 1.25}rem` }}
        >
          {hasChildren || namespace.permissions.length > 0 ? (
            isExpanded ? (
              <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            )
          ) : (
            <div className="h-4 w-4 shrink-0" />
          )}
          <span className="font-medium text-sm text-foreground capitalize">{namespace.name}</span>
          <Badge variant="secondary" className="text-xs">
            {permissionCount}
          </Badge>
        </button>

        {/* Namespace content */}
        {isExpanded && (
          <>
            {sortPermissions(namespace.permissions).map((p) => renderPermission(p, depth + 1))}
            {Array.from(namespace.children.values())
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((child) => renderNamespace(child, depth + 1))}
          </>
        )}
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
            <div key={group.moduleId} className="rounded-lg border border-border bg-card shadow-sm">
              {/* Module header */}
              <button
                onClick={() => toggleModule(group.moduleId)}
                className="flex w-full items-center justify-between border-b border-border/80 px-5 py-3 text-left hover:bg-muted/30"
              >
                <div className="flex items-center gap-3">
                  {isModuleExpanded ? (
                    <ChevronDown className="h-5 w-5 shrink-0 text-muted-foreground" />
                  ) : (
                    <ChevronRight className="h-5 w-5 shrink-0 text-muted-foreground" />
                  )}
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
              {isModuleExpanded && (
                <div className="divide-y divide-border/70">
                  {renderNamespace(group.namespaces, 0)}
                </div>
              )}
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
