import { useMemo, useState, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight, Folder, Loader2, Plus } from 'lucide-react'
import { FolderTree } from './FolderTree'
import { EmptyFolderState } from './EmptyFolderState'
import { FolderFormModal, type FolderFormMode } from './FolderFormModal'
import { DeleteFolderConfirmModal } from './DeleteFolderConfirmModal'
import { FolderContextMenu } from './FolderContextMenu'
import { cn } from '@/lib/utils/cn'
import type { ConnectionFolderNode, ConnectionFolderSummary } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import { usePermissions } from '@/hooks/usePermissions'
import { PERMISSIONS } from '@/constants/permissions'
import { useConnectionFolderMutations } from '@/hooks/useConnectionFolderMutations'
import { Button } from '@/components/ui/Button'

interface FolderSidebarProps {
  folders: ConnectionFolderNode[]
  activeFolderId: string | null
  isLoading?: boolean
  onFolderSelect: (folderId: string | null) => void
  teamId?: string | null
  teams?: TeamRecord[]
}

interface FolderFormState {
  mode: FolderFormMode
  folder?: ConnectionFolderSummary
}

export function FolderSidebar({
  folders,
  activeFolderId,
  isLoading,
  onFolderSelect,
  teamId,
  teams = [],
}: FolderSidebarProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [formState, setFormState] = useState<FolderFormState | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ConnectionFolderNode | null>(null)

  const { hasAnyPermission } = usePermissions()
  const canCreateFolders = hasAnyPermission([
    PERMISSIONS.CONNECTION_FOLDER.CREATE,
    PERMISSIONS.CONNECTION_FOLDER.MANAGE,
    PERMISSIONS.PERMISSION.MANAGE,
  ])
  const canUpdateFolders = hasAnyPermission([
    PERMISSIONS.CONNECTION_FOLDER.UPDATE,
    PERMISSIONS.CONNECTION_FOLDER.MANAGE,
    PERMISSIONS.PERMISSION.MANAGE,
  ])
  const canDeleteFolders = hasAnyPermission([
    PERMISSIONS.CONNECTION_FOLDER.DELETE,
    PERMISSIONS.CONNECTION_FOLDER.MANAGE,
    PERMISSIONS.PERMISSION.MANAGE,
  ])
  const canAssignTeams = hasAnyPermission([
    PERMISSIONS.TEAM.VIEW_ALL,
    PERMISSIONS.TEAM.MANAGE,
    PERMISSIONS.TEAM.UPDATE,
  ])

  const { remove } = useConnectionFolderMutations()

  const userFolders = useMemo(
    () => folders.filter((node) => node.folder.id !== 'unassigned'),
    [folders]
  )
  const hasUserFolders = userFolders.length > 0

  const handleOpenCreate = () => {
    if (!canCreateFolders) {
      return
    }
    setFormState({
      mode: 'create',
    })
  }

  const handleOpenEdit = (folder: ConnectionFolderSummary) => {
    if (!canUpdateFolders) {
      return
    }
    setFormState({
      mode: 'edit',
      folder,
    })
  }

  const handleDelete = (node: ConnectionFolderNode) => {
    if (!canDeleteFolders) {
      return
    }
    setDeleteTarget(node)
  }

  const closeForm = () => setFormState(null)

  const handleFormSuccess = (folder: ConnectionFolderSummary) => {
    onFolderSelect(folder.id)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) {
      return
    }
    try {
      await remove.mutateAsync(deleteTarget.folder.id)
      if (activeFolderId === deleteTarget.folder.id) {
        onFolderSelect(null)
      }
      setDeleteTarget(null)
    } catch {
      // Errors surface via toast; keep modal open to retry.
    }
  }

  return (
    <>
      <div
        className={cn(
          'shrink-0 transition-all duration-300 ease-in-out',
          collapsed ? 'w-16' : 'w-72'
        )}
      >
        <div className="flex h-full flex-col overflow-hidden rounded-lg border border-border/60 bg-card shadow-sm">
          {/* Header */}
          <div
            className={cn(
              'flex items-center justify-between border-b border-border/60 p-4 transition-all',
              collapsed && 'flex-col gap-2 p-3'
            )}
          >
            <div className={cn('flex items-center gap-2.5', collapsed && 'flex-col gap-1.5')}>
              <Folder className="h-4 w-4 shrink-0 text-muted-foreground" />
              {!collapsed && (
                <h2 className="text-sm font-semibold uppercase leading-none tracking-wide">
                  Folders
                </h2>
              )}
            </div>
            <div className="flex items-center gap-2">
              {canCreateFolders ? (
                collapsed ? (
                  <button
                    type="button"
                    onClick={() => handleOpenCreate()}
                    className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                    aria-label="Create folder"
                  >
                    <Plus className="h-4 w-4" />
                  </button>
                ) : (
                  <Button size="sm" onClick={() => handleOpenCreate()}>
                    <Plus className="mr-1.5 h-3.5 w-3.5" />
                    New Folder
                  </Button>
                )
              ) : null}
              {isLoading && (
                <Loader2 className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
              )}
              <button
                onClick={() => setCollapsed(!collapsed)}
                className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                aria-label={collapsed ? 'Expand folders' : 'Collapse folders'}
              >
                {collapsed ? (
                  <ChevronRight className="h-4 w-4" />
                ) : (
                  <ChevronLeft className="h-4 w-4" />
                )}
              </button>
            </div>
          </div>

          {/* Content */}
          <div className={cn('flex-1 overflow-y-auto', collapsed ? 'p-2' : 'p-3')}>
            {collapsed ? (
              <CollapsedFolderList
                folders={folders}
                activeFolderId={activeFolderId}
                onSelect={onFolderSelect}
              />
            ) : hasUserFolders ? (
              <>
                <button
                  onClick={() => onFolderSelect(null)}
                  className={cn(
                    'mb-2 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
                    !activeFolderId
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                  )}
                >
                  <Folder className="h-4 w-4" />
                  <span>All Folders</span>
                </button>
                <FolderTree
                  nodes={folders}
                  activeFolderId={activeFolderId}
                  onSelect={onFolderSelect}
                  renderActions={(node) => {
                    const isUnassigned = node.folder.id === 'unassigned'
                    const allowEdit = canUpdateFolders && !isUnassigned
                    const allowDelete = canDeleteFolders && !isUnassigned
                    if (!allowEdit && !allowDelete) {
                      return null
                    }
                    return (
                      <FolderContextMenu
                        folder={node.folder}
                        canEdit={allowEdit}
                        canDelete={allowDelete}
                        disabled={!allowEdit && !allowDelete}
                        onEdit={handleOpenEdit}
                        onDelete={() => handleDelete(node)}
                      />
                    )
                  }}
                />
              </>
            ) : (
              <div className="space-y-3">
                <EmptyFolderState
                  canCreateFolders={canCreateFolders}
                  onCreateFolder={handleOpenCreate}
                />
                {folders.length ? (
                  <>
                    <button
                      onClick={() => onFolderSelect(null)}
                      className={cn(
                        'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
                        !activeFolderId
                          ? 'bg-muted text-foreground'
                          : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                      )}
                    >
                      <Folder className="h-4 w-4" />
                      <span>Unassigned</span>
                    </button>
                    <FolderTree
                      nodes={folders}
                      activeFolderId={activeFolderId}
                      onSelect={onFolderSelect}
                    />
                  </>
                ) : null}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Folder form */}
      {formState ? (
        <FolderFormModal
          open={Boolean(formState)}
          mode={formState.mode}
          folder={formState.folder}
          onClose={closeForm}
          onSuccess={handleFormSuccess}
          teamId={teamId ?? null}
          teams={teams}
          allowTeamAssignment={canAssignTeams}
        />
      ) : null}

      {/* Delete confirmation */}
      <DeleteFolderConfirmModal
        open={Boolean(deleteTarget)}
        folder={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDeleteConfirm}
      />
    </>
  )
}

function CollapsedFolderList({
  folders,
  activeFolderId,
  onSelect,
}: {
  folders: ConnectionFolderNode[]
  activeFolderId: string | null
  onSelect: (folderId: string | null) => void
}) {
  return (
    <div className="space-y-2">
      <CollapsedFolderButton
        label="All Folders"
        isActive={!activeFolderId}
        onClick={() => onSelect(null)}
      />
      {folders.map((node) => (
        <CollapsedFolderNode
          key={node.folder.id}
          node={node}
          activeFolderId={activeFolderId}
          onSelect={onSelect}
        />
      ))}
    </div>
  )
}

function CollapsedFolderNode({
  node,
  activeFolderId,
  onSelect,
}: {
  node: ConnectionFolderNode
  activeFolderId: string | null
  onSelect: (folderId: string | null) => void
}) {
  const isActive = activeFolderId === node.folder.id

  return (
    <>
      <CollapsedFolderButton
        label={node.folder.name}
        isActive={isActive}
        onClick={() => onSelect(node.folder.id === 'unassigned' ? null : node.folder.id)}
        icon={<Folder className="h-4 w-4" />}
      />
      {node.children?.map((child) => (
        <CollapsedFolderNode
          key={child.folder.id}
          node={child}
          activeFolderId={activeFolderId}
          onSelect={onSelect}
        />
      ))}
    </>
  )
}

function CollapsedFolderButton({
  label,
  isActive,
  onClick,
  icon,
}: {
  label: string
  isActive: boolean
  onClick: () => void
  icon?: ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex h-10 w-full items-center justify-center rounded-md transition-colors',
        isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted'
      )}
      title={label}
    >
      {icon ?? <Folder className="h-4 w-4" />}
    </button>
  )
}
