import { useMemo, useState } from 'react'
import { Search, ShieldAlert } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { cn } from '@/lib/utils/cn'
import {
  findPermissionDependents,
  groupPermissionsByModule,
  resolvePermissionDependencies,
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

interface PermissionGroup {
  moduleId: string
  permissions: PermissionDefinition[]
}

const HUMANIZED_MODULES: Record<string, string> = {
  core: 'Core Platform',
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

export function PermissionMatrix({
  registry,
  loading,
  selected,
  onChange,
  disabled,
}: PermissionMatrixProps) {
  const [searchTerm, setSearchTerm] = useState('')

  const normalisedSearch = searchTerm.trim().toLowerCase()

  const groups = useMemo<PermissionGroup[]>(() => {
    if (!registry) {
      return []
    }

    const grouped = groupPermissionsByModule(registry)
    return Object.entries(grouped)
      .map(([moduleId, permissions]) => ({
        moduleId,
        permissions: sortPermissions(permissions).filter((permission) =>
          matchesSearch(permission, normalisedSearch)
        ),
      }))
      .filter((group) => group.permissions.length > 0)
      .sort((a, b) => a.moduleId.localeCompare(b.moduleId))
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

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <Badge variant="outline" className="font-medium">
            {selected.size} selected
          </Badge>
          <span>out of {totalPermissions} permissions</span>
        </div>
        <div className="relative w-full max-w-sm">
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

      <div className="space-y-5">
        {groups.map((group) => (
          <div key={group.moduleId} className="rounded-lg border border-border bg-card shadow-sm">
            <div className="flex items-center justify-between border-b border-border/80 px-5 py-3">
              <div>
                <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                  {HUMANIZED_MODULES[group.moduleId] ?? group.moduleId}
                </h3>
                <p className="text-xs text-muted-foreground/80">
                  {group.permissions.length} permission
                  {group.permissions.length === 1 ? '' : 's'} visible
                </p>
              </div>
            </div>

            <div className="divide-y divide-border/70">
              {group.permissions.map((permission) => {
                const isChecked = selected.has(permission.id)
                const dependents = dependentIndex[permission.id] ?? []
                const blockingDependents = dependents.filter((dep) => selected.has(dep))
                const isLocked = !disabled && isChecked && blockingDependents.length > 0

                return (
                  <div
                    key={permission.id}
                    className={cn(
                      'flex flex-col gap-2 px-5 py-4 md:flex-row md:items-start md:justify-between',
                      isChecked ? 'bg-primary/5' : 'hover:bg-muted/40'
                    )}
                  >
                    <div className="flex flex-1 items-start gap-3">
                      <Checkbox
                        checked={isChecked}
                        onCheckedChange={(checked) => handleToggle(permission.id, Boolean(checked))}
                        disabled={disabled || (isChecked && isLocked)}
                        aria-label={permission.id}
                        aria-describedby={`${permission.id}-details`}
                      />
                      <div>
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
                          <p className="mt-1 text-sm text-muted-foreground">
                            {permission.description}
                          </p>
                        ) : null}
                        <div className="mt-2 space-y-2 text-xs text-muted-foreground">
                          {permission.depends_on?.length ? (
                            <div
                              className="flex flex-wrap items-center gap-1"
                              id={`${permission.id}-details`}
                            >
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
              })}
            </div>
          </div>
        ))}

        {groups.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border bg-muted/10 p-8 text-center text-sm text-muted-foreground">
            No permissions match your search.
          </div>
        ) : null}
      </div>
    </div>
  )
}
