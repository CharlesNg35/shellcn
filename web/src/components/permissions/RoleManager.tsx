import { useMemo, useState } from 'react'
import { Plus, ShieldCheck, Trash2, PencilLine, Filter, Search, Layers } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { ListItemSkeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import type { RoleRecord } from '@/types/permission'
import { PERMISSIONS } from '@/constants/permissions'
import { cn } from '@/lib/utils/cn'

type RoleFilter = 'all' | 'system' | 'custom'

interface RoleManagerProps {
  roles: RoleRecord[] | undefined
  selectedRoleId?: string
  onSelectRole: (roleId: string) => void
  onCreateRole: () => void
  onEditRole: (role: RoleRecord) => void
  onDeleteRole: (role: RoleRecord) => void
  isLoading?: boolean
}

const FILTER_OPTIONS: Array<{ id: RoleFilter; label: string }> = [
  { id: 'all', label: 'All roles' },
  { id: 'system', label: 'System' },
  { id: 'custom', label: 'Custom' },
]

export function RoleManager({
  roles,
  selectedRoleId,
  onSelectRole,
  onCreateRole,
  onEditRole,
  onDeleteRole,
  isLoading,
}: RoleManagerProps) {
  const [filter, setFilter] = useState<RoleFilter>('all')
  const [search, setSearch] = useState('')

  const filteredRoles = useMemo(() => {
    if (!roles) {
      return []
    }
    const searchTerm = search.trim().toLowerCase()
    return roles
      .filter((role) => {
        if (filter === 'system' && !role.is_system) {
          return false
        }
        if (filter === 'custom' && role.is_system) {
          return false
        }
        if (!searchTerm) {
          return true
        }
        const haystack = `${role.name} ${role.description ?? ''}`.toLowerCase()
        return haystack.includes(searchTerm)
      })
      .sort((a, b) => {
        if (a.is_system && !b.is_system) {
          return -1
        }
        if (!a.is_system && b.is_system) {
          return 1
        }
        return a.name.localeCompare(b.name)
      })
  }, [roles, filter, search])

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Filter className="h-4 w-4" />
            <span>Filter roles</span>
          </div>
          <PermissionGuard permission={PERMISSIONS.PERMISSION.MANAGE}>
            <Button size="sm" onClick={onCreateRole}>
              <Plus className="mr-1 h-4 w-4" />
              New Role
            </Button>
          </PermissionGuard>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {FILTER_OPTIONS.map((option) => (
            <Button
              key={option.id}
              type="button"
              variant={filter === option.id ? 'default' : 'outline'}
              size="sm"
              onClick={() => setFilter(option.id)}
            >
              {option.label}
            </Button>
          ))}
        </div>

        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search roles..."
            className="pl-9"
            aria-label="Search roles"
          />
        </div>
      </div>

      <div className="flex-1 overflow-hidden rounded-lg border border-border bg-card shadow-sm">
        <div className="flex items-center justify-between border-b border-border/80 px-4 py-3">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Layers className="h-4 w-4" />
            <span>
              {filteredRoles.length} role{filteredRoles.length === 1 ? '' : 's'}
            </span>
          </div>
        </div>

        <div className="max-h-[520px] overflow-y-auto">
          {isLoading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 5 }).map((_, index) => (
                <ListItemSkeleton key={index} />
              ))}
            </div>
          ) : filteredRoles.length === 0 ? (
            <EmptyState
              className="border-0"
              icon={ShieldCheck}
              title="No roles found"
              description="Adjust filters or create a new role to get started."
              action={
                <PermissionGuard permission={PERMISSIONS.PERMISSION.MANAGE}>
                  <Button onClick={onCreateRole}>
                    <Plus className="mr-1 h-4 w-4" />
                    Create Role
                  </Button>
                </PermissionGuard>
              }
            />
          ) : (
            <ul className="divide-y divide-border/70">
              {filteredRoles.map((role) => {
                const isSelected = role.id === selectedRoleId
                const permissionCount = role.permissions?.length ?? 0

                return (
                  <li
                    key={role.id}
                    className={cn(
                      'group relative flex cursor-pointer items-center justify-between px-4 py-3 hover:bg-primary/5',
                      isSelected && 'bg-primary/10'
                    )}
                    onClick={() => onSelectRole(role.id)}
                  >
                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium text-foreground">{role.name}</span>
                        {role.is_system ? (
                          <Badge variant="outline" className="flex items-center gap-1 text-xs">
                            <ShieldCheck className="h-3 w-3" />
                            System
                          </Badge>
                        ) : null}
                      </div>
                      {role.description ? (
                        <p className="truncate text-sm text-muted-foreground">{role.description}</p>
                      ) : null}
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>
                          {permissionCount} permission{permissionCount === 1 ? '' : 's'}
                        </span>
                        {role.created_at ? (
                          <span>
                            Created{' '}
                            {new Date(role.created_at).toLocaleDateString(undefined, {
                              year: 'numeric',
                              month: 'short',
                              day: 'numeric',
                            })}
                          </span>
                        ) : null}
                      </div>
                    </div>

                    {!role.is_system ? (
                      <PermissionGuard permission={PERMISSIONS.PERMISSION.MANAGE}>
                        <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            onClick={(event) => {
                              event.stopPropagation()
                              onEditRole(role)
                            }}
                            aria-label={`Edit ${role.name}`}
                          >
                            <PencilLine className="h-4 w-4" />
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            onClick={(event) => {
                              event.stopPropagation()
                              onDeleteRole(role)
                            }}
                            className="text-destructive hover:text-destructive"
                            aria-label={`Delete ${role.name}`}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </PermissionGuard>
                    ) : null}
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
