import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, ShieldCheck } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { RoleManager } from '@/components/permissions/RoleManager'
import { PermissionMatrix } from '@/components/permissions/PermissionMatrix'
import { Modal } from '@/components/ui/Modal'
import { RoleForm, type RoleFormMode } from '@/components/permissions/RoleForm'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { Badge } from '@/components/ui/Badge'
import { usePermissionRegistry } from '@/hooks/usePermissionRegistry'
import { useRoles, useRoleMutations } from '@/hooks/useRoles'
import { usePermissions } from '@/hooks/usePermissions'
import type { PermissionIdentifier, RoleRecord } from '@/types/permission'
import { PERMISSIONS } from '@/constants/permissions'

function setsEqual<T>(a: Set<T>, b: Set<T>): boolean {
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

export function Permissions() {
  const {
    data: registry,
    isLoading: isRegistryLoading,
    error: registryError,
  } = usePermissionRegistry()
  const { data: roles, isLoading: isRolesLoading, error: rolesError } = useRoles()
  const { hasPermission } = usePermissions()
  const { deleteRole, setRolePermissions } = useRoleMutations()

  const [selectedRoleId, setSelectedRoleId] = useState<string | undefined>(undefined)
  const [selectedPermissions, setSelectedPermissions] = useState<Set<PermissionIdentifier>>(
    () => new Set()
  )
  const [isRoleModalOpen, setRoleModalOpen] = useState(false)
  const [roleFormMode, setRoleFormMode] = useState<RoleFormMode>('create')
  const [activeRole, setActiveRole] = useState<RoleRecord | undefined>(undefined)

  const canManagePermissions = hasPermission(PERMISSIONS.PERMISSION.MANAGE)

  const selectedRole = useMemo(
    () => roles?.find((role) => role.id === selectedRoleId),
    [roles, selectedRoleId]
  )

  useEffect(() => {
    if (!roles || roles.length === 0) {
      setSelectedRoleId(undefined)
      return
    }

    if (!selectedRoleId || !roles.some((role) => role.id === selectedRoleId)) {
      setSelectedRoleId(roles[0].id)
    }
  }, [roles, selectedRoleId])

  useEffect(() => {
    if (!selectedRole) {
      setSelectedPermissions(new Set())
      return
    }

    const baseline = new Set<PermissionIdentifier>(
      (selectedRole.permissions ?? []).map((permission) => permission.id)
    )

    setSelectedPermissions((current) => {
      if (setsEqual(current, baseline)) {
        return current
      }
      return baseline
    })
  }, [selectedRole])

  const baselinePermissions = useMemo(() => {
    return new Set<PermissionIdentifier>(
      (selectedRole?.permissions ?? []).map((permission) => permission.id)
    )
  }, [selectedRole])

  const isSystemRole = selectedRole?.is_system ?? false
  const isDirty = selectedRole ? !setsEqual(selectedPermissions, baselinePermissions) : false
  const isSavingPermissions = setRolePermissions.isPending

  const handleOpenCreateRole = () => {
    setActiveRole(undefined)
    setRoleFormMode('create')
    setRoleModalOpen(true)
  }

  const handleOpenEditRole = (role: RoleRecord) => {
    setActiveRole(role)
    setRoleFormMode('edit')
    setRoleModalOpen(true)
  }

  const handleRoleFormClose = () => {
    setRoleModalOpen(false)
    setActiveRole(undefined)
  }

  const handleRoleFormSuccess = (role: RoleRecord) => {
    setRoleModalOpen(false)
    setActiveRole(undefined)
    setSelectedRoleId(role.id)
  }

  const handleRoleSelection = (roleId: string) => {
    setSelectedRoleId(roleId)
  }

  const handlePermissionsChange = (next: PermissionIdentifier[]) => {
    setSelectedPermissions(new Set(next))
  }

  const handleResetPermissions = () => {
    setSelectedPermissions(new Set(baselinePermissions))
  }

  const handleSavePermissions = async () => {
    if (!selectedRole) {
      return
    }

    await setRolePermissions
      .mutateAsync({
        roleId: selectedRole.id,
        roleName: selectedRole.name,
        payload: {
          permissions: Array.from(selectedPermissions),
        },
      })
      .catch(() => undefined)
  }

  const handleDeleteRole = async (role: RoleRecord) => {
    if (deleteRole.isPending) {
      return
    }

    const confirmed = window.confirm(
      `Delete role "${role.name}"? This will remove role assignments from users.`
    )
    if (!confirmed) {
      return
    }

    await deleteRole.mutateAsync({ roleId: role.id, roleName: role.name }).catch(() => undefined)

    if (selectedRoleId === role.id) {
      setSelectedRoleId(undefined)
    }
  }

  const showRegistryError = Boolean(registryError)
  const showRolesError = Boolean(rolesError)

  const registryErrorMessage = registryError?.message ?? 'Failed to load permission registry.'
  const rolesErrorMessage = rolesError?.message ?? 'Failed to load roles.'

  const roleCount = roles?.length ?? 0
  const isRolesInitialLoading = isRolesLoading && roleCount === 0

  return (
    <div className="space-y-6">
      <PageHeader
        title="Permissions"
        description="Review role assignments and manage fine-grained permissions across the platform. Changes take effect immediately for all users assigned to a role."
      />

      {showRegistryError ? (
        <div className="flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <AlertTriangle className="h-4 w-4" />
          <span>{registryErrorMessage}</span>
        </div>
      ) : null}

      {showRolesError ? (
        <div className="flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <AlertTriangle className="h-4 w-4" />
          <span>{rolesErrorMessage}</span>
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[360px_1fr]">
        <RoleManager
          roles={roles}
          selectedRoleId={selectedRoleId}
          onSelectRole={handleRoleSelection}
          onCreateRole={handleOpenCreateRole}
          onEditRole={handleOpenEditRole}
          onDeleteRole={handleDeleteRole}
          isLoading={isRolesInitialLoading}
        />

        <div className="space-y-5">
          {selectedRole ? (
            <>
              <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-semibold text-foreground">{selectedRole.name}</h2>
                    <p className="text-sm text-muted-foreground">
                      {selectedRole.description ||
                        'Assign functional access to this role by selecting permissions below.'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline">
                      {(selectedRole.permissions ?? []).length} permissions
                    </Badge>
                    {selectedRole.is_system ? (
                      <Badge variant="outline" className="flex items-center gap-1">
                        <ShieldCheck className="h-3 w-3" />
                        System role
                      </Badge>
                    ) : null}
                  </div>
                </div>

                {(!canManagePermissions || isSystemRole) && (
                  <div className="mt-4 flex items-start gap-2 rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                    <span>
                      {isSystemRole
                        ? 'System roles are immutable to preserve baseline platform access.'
                        : 'You have read-only access. Contact an administrator to request permission updates.'}
                    </span>
                  </div>
                )}
              </div>

              <PermissionMatrix
                registry={registry}
                loading={isRegistryLoading}
                selected={selectedPermissions}
                onChange={handlePermissionsChange}
                disabled={!canManagePermissions || isSystemRole || isSavingPermissions}
              />

              {canManagePermissions && !isSystemRole ? (
                <div className="flex justify-end gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleResetPermissions}
                    disabled={!isDirty || isSavingPermissions}
                  >
                    Reset
                  </Button>
                  <Button
                    type="button"
                    onClick={handleSavePermissions}
                    disabled={!isDirty || isSavingPermissions}
                    loading={isSavingPermissions}
                  >
                    Save Changes
                  </Button>
                </div>
              ) : null}
            </>
          ) : (
            <EmptyState
              className="min-h-[420px]"
              icon={ShieldCheck}
              title="Select a role"
              description="Choose a role from the list to review or adjust its permissions."
            />
          )}
        </div>
      </div>

      <Modal
        open={isRoleModalOpen}
        onClose={handleRoleFormClose}
        title={roleFormMode === 'create' ? 'Create Role' : `Edit ${activeRole?.name ?? 'Role'}`}
        description={
          roleFormMode === 'create'
            ? 'Define a reusable permission bundle that can be assigned to multiple users.'
            : 'Update role metadata. System role names are locked to preserve compatibility.'
        }
      >
        <RoleForm
          mode={roleFormMode}
          role={activeRole}
          onClose={handleRoleFormClose}
          onSuccess={handleRoleFormSuccess}
        />
      </Modal>
    </div>
  )
}
