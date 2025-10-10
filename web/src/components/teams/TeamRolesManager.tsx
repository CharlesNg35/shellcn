import { useEffect, useMemo, useState } from 'react'
import { Loader2, ShieldCheck } from 'lucide-react'
import type { UseMutationResult } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Checkbox } from '@/components/ui/Checkbox'
import type { RoleRecord } from '@/types/permission'
import type { UserRoleSummary } from '@/types/users'
import type { ApiError } from '@/lib/api/http'

interface TeamRolesManagerProps {
  teamId: string
  teamName: string
  assignedRoles: UserRoleSummary[]
  availableRoles?: RoleRecord[]
  isLoadingRoles?: boolean
  setRolesMutation: UseMutationResult<
    UserRoleSummary[],
    ApiError,
    { teamId: string; roleIds: string[] }
  >
  canManageRoles: boolean
}

function toSet(values: string[]): Set<string> {
  return new Set(values)
}

function areSelectionsEqual(a: string[], b: Set<string>): boolean {
  if (a.length !== b.size) {
    return false
  }
  for (const value of a) {
    if (!b.has(value)) {
      return false
    }
  }
  return true
}

export function TeamRolesManager({
  teamId,
  teamName,
  assignedRoles,
  availableRoles,
  isLoadingRoles,
  setRolesMutation,
  canManageRoles,
}: TeamRolesManagerProps) {
  const baseline = useMemo(() => toSet(assignedRoles.map((role) => role.id)), [assignedRoles])
  const [selection, setSelection] = useState<string[]>(() => Array.from(baseline))

  useEffect(() => {
    setSelection(Array.from(baseline))
  }, [baseline])

  const sortedRoles = useMemo(() => {
    return [...(availableRoles ?? [])].sort((a, b) => a.name.localeCompare(b.name))
  }, [availableRoles])

  const toggleSelection = (roleId: string) => {
    setSelection((current) => {
      if (current.includes(roleId)) {
        return current.filter((id) => id !== roleId)
      }
      return [...current, roleId]
    })
  }

  const handleReset = () => {
    setSelection(Array.from(baseline))
  }

  const handleSave = async () => {
    try {
      await setRolesMutation.mutateAsync({
        teamId,
        roleIds: selection,
      })
    } catch {
      // handled by mutation toast
    }
  }

  const isSaving = setRolesMutation.isPending
  const isDirty = !areSelectionsEqual(selection, baseline)
  const disabled = !canManageRoles || isSaving

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="flex items-center justify-between border-b border-border/70 pb-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Team Roles</h3>
          <p className="text-xs text-muted-foreground">
            Roles assigned here automatically apply to every member of {teamName}.
          </p>
        </div>
        <Badge variant="outline" className="text-[10px] uppercase tracking-wide">
          {assignedRoles.length} assigned
        </Badge>
      </div>

      {isLoadingRoles ? (
        <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Loading roles…
        </div>
      ) : sortedRoles.length === 0 ? (
        <div className="py-6 text-sm text-muted-foreground">
          No roles available. Create roles in the Permissions section first.
        </div>
      ) : (
        <div className="space-y-4 pt-4">
          <div className="space-y-2">
            {sortedRoles.map((role) => {
              const checked = selection.includes(role.id)
              return (
                <label
                  key={role.id}
                  className="flex cursor-pointer items-start gap-3 rounded-lg border border-border/60 bg-background px-3 py-2 hover:border-border"
                >
                  <Checkbox
                    checked={checked}
                    onCheckedChange={() => toggleSelection(role.id)}
                    disabled={disabled}
                  />
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-foreground">{role.name}</span>
                      {role.is_system ? (
                        <Badge variant="outline" className="flex items-center gap-1 text-[10px]">
                          <ShieldCheck className="h-3 w-3" /> System
                        </Badge>
                      ) : null}
                      {checked ? (
                        <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                          Assigned
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {role.description || 'No description provided.'}
                    </p>
                  </div>
                </label>
              )
            })}
          </div>

          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={handleReset}
              disabled={!isDirty || disabled}
            >
              Reset
            </Button>
            <Button
              type="button"
              onClick={handleSave}
              disabled={!isDirty || !canManageRoles}
              loading={isSaving}
            >
              Save Changes
            </Button>
          </div>

          {!canManageRoles ? (
            <p className="text-xs text-muted-foreground">
              You have read-only access. Contact an administrator with permission management rights
              to adjust team roles.
            </p>
          ) : null}
        </div>
      )}
    </div>
  )
}
