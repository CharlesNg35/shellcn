import { useEffect, useMemo, useState, useCallback } from 'react'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PageHeader } from '@/components/layout/PageHeader'
import { UserFilters, normalizeFilters, type UserFilterState } from '@/components/users/UserFilters'
import { UserTable } from '@/components/users/UserTable'
import { UserForm } from '@/components/users/UserForm'
import { UserDetailModal } from '@/components/users/UserDetailModal'
import { UserBulkActionsBar } from '@/components/users/UserBulkActionsBar'
import { useUserMutations, useUsers } from '@/hooks/useUsers'
import type { UserRecord } from '@/types/users'
import { PERMISSIONS } from '@/constants/permissions'

const DEFAULT_PER_PAGE = 20

export function Users() {
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<UserFilterState>({ status: 'all' })
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [detailUserId, setDetailUserId] = useState<string | undefined>(undefined)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null)

  const queryParams = useMemo(() => {
    return {
      ...normalizeFilters(filters),
      page,
      per_page: DEFAULT_PER_PAGE,
    }
  }, [filters, page])

  const { data, isLoading, refetch } = useUsers(queryParams)
  const { bulkActivate, bulkDeactivate, bulkDelete } = useUserMutations()

  useEffect(() => {
    setPage(1)
  }, [filters.status, filters.search])

  const users = data?.data ?? []
  const meta = data?.meta

  const handleBulkActivate = async () => {
    if (!selectedIds.length) {
      return
    }
    await bulkActivate.mutateAsync({ user_ids: selectedIds })
    setSelectedIds([])
  }

  const handleBulkDeactivate = async () => {
    if (!selectedIds.length) {
      return
    }
    await bulkDeactivate.mutateAsync({ user_ids: selectedIds })
    setSelectedIds([])
  }

  const handleBulkDelete = async () => {
    if (!selectedIds.length) {
      return
    }
    await bulkDelete.mutateAsync({ user_ids: selectedIds })
    setSelectedIds([])
  }

  const isBulkProcessing =
    bulkActivate.isPending || bulkDeactivate.isPending || bulkDelete.isPending

  const handleCreateSuccess = () => {
    setIsCreateModalOpen(false)
    refetch()
  }

  const handleEditSuccess = (updated: UserRecord) => {
    setEditingUser(null)
    if (detailUserId === updated.id) {
      setDetailUserId(updated.id)
    }
    refetch()
  }

  const handleViewUser = useCallback((record: UserRecord) => {
    setDetailUserId(record.id)
  }, [])

  const handleEditUser = useCallback((record: UserRecord) => {
    setEditingUser(record)
  }, [])

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        description="Manage platform users, activation status, and administrative privileges. Create new accounts, assign roles, and control access to system resources."
        action={
          <PermissionGuard permission={PERMISSIONS.USER.CREATE}>
            <Button onClick={() => setIsCreateModalOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Create User
            </Button>
          </PermissionGuard>
        }
      />

      <div className="space-y-4">
        <UserFilters filters={filters} onChange={setFilters} />

        <UserBulkActionsBar
          selectedCount={selectedIds.length}
          onActivate={handleBulkActivate}
          onDeactivate={handleBulkDeactivate}
          onDelete={handleBulkDelete}
          isProcessing={isBulkProcessing}
        />

        <UserTable
          users={users}
          meta={meta}
          page={meta?.page ?? page}
          perPage={meta?.per_page ?? DEFAULT_PER_PAGE}
          isLoading={isLoading}
          onPageChange={setPage}
          onSelectionChange={setSelectedIds}
          onViewUser={handleViewUser}
          onEditUser={handleEditUser}
        />
      </div>

      <Modal
        open={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        title="Create User"
        description="Add a new user account to the platform. Set initial credentials, status, and administrative privileges."
      >
        <UserForm
          mode="create"
          onClose={() => setIsCreateModalOpen(false)}
          onSuccess={handleCreateSuccess}
        />
      </Modal>

      <Modal
        open={Boolean(editingUser)}
        onClose={() => setEditingUser(null)}
        title={editingUser ? `Edit ${editingUser.username}` : 'Edit User'}
        description="Update user account details, status, and permissions. Changes take effect immediately."
      >
        <UserForm
          mode="edit"
          user={editingUser ?? undefined}
          onClose={() => setEditingUser(null)}
          onSuccess={handleEditSuccess}
        />
      </Modal>

      <UserDetailModal
        userId={detailUserId}
        open={Boolean(detailUserId)}
        onClose={() => setDetailUserId(undefined)}
        onEdit={(record) => {
          setEditingUser(record)
          setDetailUserId(undefined)
        }}
      />
    </div>
  )
}
