import { useEffect, useMemo, useState } from 'react'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { UserFilters, normalizeFilters, type UserFilterState } from '@/components/users/UserFilters'
import { UserTable } from '@/components/users/UserTable'
import { UserForm } from '@/components/users/UserForm'
import { UserDetailModal } from '@/components/users/UserDetailModal'
import { UserBulkActionsBar } from '@/components/users/UserBulkActionsBar'
import { Modal } from '@/components/ui/Modal'
import { useUserMutations, useUsers } from '@/hooks/useUsers'
import type { UserRecord } from '@/types/users'

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

  const { data, isLoading } = useUsers(queryParams)
  const { bulkActivate, bulkDeactivate, bulkDelete } = useUserMutations()

  useEffect(() => {
    setPage(1)
  }, [filters.status, filters.search, filters.organization_id])

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

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-3xl font-bold text-foreground">Users</h1>
          <p className="text-sm text-muted-foreground">
            Manage platform users, activation status, and administrative privileges
          </p>
        </div>
        <PermissionGuard permission="user.create">
          <Button onClick={() => setIsCreateModalOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create user
          </Button>
        </PermissionGuard>
      </div>

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
        onViewUser={(record) => setDetailUserId(record.id)}
        onEditUser={(record) => setEditingUser(record)}
      />

      <Modal
        open={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        title="Create user"
      >
        <UserForm
          mode="create"
          onClose={() => setIsCreateModalOpen(false)}
          onSuccess={() => setIsCreateModalOpen(false)}
        />
      </Modal>

      <Modal
        open={Boolean(editingUser)}
        onClose={() => setEditingUser(null)}
        title={editingUser ? `Edit ${editingUser.username}` : 'Edit user'}
      >
        <UserForm
          mode="edit"
          user={editingUser ?? undefined}
          onClose={() => setEditingUser(null)}
          onSuccess={(updated) => {
            setEditingUser(null)
            if (detailUserId === updated.id) {
              setDetailUserId(updated.id)
            }
          }}
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
