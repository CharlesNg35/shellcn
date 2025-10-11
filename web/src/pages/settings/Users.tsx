import { useEffect, useMemo, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, UserPlus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PageHeader } from '@/components/layout/PageHeader'
import { UserFilters, normalizeFilters, type UserFilterState } from '@/components/users/UserFilters'
import { UserTable } from '@/components/users/UserTable'
import { UserForm } from '@/components/users/UserForm'
import { UserDetailModal } from '@/components/users/UserDetailModal'
import { UserBulkActionsBar } from '@/components/users/UserBulkActionsBar'
import { UserInviteForm } from '@/components/users/UserInviteForm'
import { UserInviteList } from '@/components/users/UserInviteList'
import { useUserMutations, useUsers } from '@/hooks/useUsers'
import { useInvites, useInviteMutations } from '@/hooks/useInvites'
import type { UserRecord } from '@/types/users'
import { PERMISSIONS } from '@/constants/permissions'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'

const DEFAULT_PER_PAGE = 20

export function Users() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<UserFilterState>({ status: 'all' })
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [detailUserId, setDetailUserId] = useState<string | undefined>(undefined)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null)
  const [activeTab, setActiveTab] = useState<'users' | 'invites'>('users')

  const queryParams = useMemo(() => {
    return {
      ...normalizeFilters(filters),
      page,
      per_page: DEFAULT_PER_PAGE,
    }
  }, [filters, page])

  const { data, isLoading, refetch } = useUsers(queryParams)
  const { bulkActivate, bulkDeactivate, bulkDelete } = useUserMutations()
  const invitesQuery = useInvites()
  const { remove: revokeInvite } = useInviteMutations()
  const isRevokingInvite = useCallback(
    (inviteId: string) =>
      revokeInvite.isPending && (revokeInvite.variables as string | undefined) === inviteId,
    [revokeInvite.isPending, revokeInvite.variables]
  )

  useEffect(() => {
    setPage(1)
  }, [filters.status, filters.search])

  const users = data?.data ?? []
  const meta = data?.meta
  const userCount = meta?.total ?? users.length
  const inviteCount = invitesQuery.data?.length ?? 0

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
          <div className="flex flex-wrap gap-2">
            <PermissionGuard permission={PERMISSIONS.PERMISSION.MANAGE}>
              <Button variant="ghost" onClick={() => navigate('/settings/permissions')}>
                Manage Roles
              </Button>
            </PermissionGuard>
            <PermissionGuard permission={PERMISSIONS.USER.INVITE}>
              <Button variant="outline" onClick={() => setIsInviteModalOpen(true)}>
                <UserPlus className="mr-2 h-4 w-4" />
                Invite User
              </Button>
            </PermissionGuard>
            <PermissionGuard permission={PERMISSIONS.USER.CREATE}>
              <Button onClick={() => setIsCreateModalOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Create User
              </Button>
            </PermissionGuard>
          </div>
        }
      />

      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as 'users' | 'invites')}>
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <TabsList className="gap-2">
              <TabsTrigger value="users" className="gap-2">
                <span>Users</span>
                <Badge
                  variant={activeTab === 'users' ? 'secondary' : 'outline'}
                  className="text-xs font-semibold"
                >
                  {userCount}
                </Badge>
              </TabsTrigger>
              <TabsTrigger value="invites" className="gap-2">
                <span>Invitations</span>
                <Badge
                  variant={activeTab === 'invites' ? 'secondary' : 'outline'}
                  className="text-xs font-semibold"
                >
                  {invitesQuery.isLoading ? '...' : inviteCount}
                </Badge>
              </TabsTrigger>
            </TabsList>
            <p className="text-sm text-muted-foreground">
              {activeTab === 'users'
                ? 'Review user accounts, filters, and bulk actions.'
                : 'Monitor pending invitations and revoke access when needed.'}
            </p>
          </div>

          <TabsContent value="users" className="space-y-4">
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
          </TabsContent>

          <TabsContent value="invites" className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Track pending invitations and revoke them when they are no longer needed.
            </p>
            <UserInviteList
              invites={invitesQuery.data}
              isLoading={invitesQuery.isLoading}
              onRevoke={(inviteId) => revokeInvite.mutate(inviteId)}
              isRevoking={isRevokingInvite}
            />
          </TabsContent>
        </div>
      </Tabs>

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

      <Modal
        open={isInviteModalOpen}
        onClose={() => setIsInviteModalOpen(false)}
        title="Invite User"
        description="Send an invitation link so the recipient can create their own credentials."
      >
        <UserInviteForm onClose={() => setIsInviteModalOpen(false)} />
      </Modal>
    </div>
  )
}
