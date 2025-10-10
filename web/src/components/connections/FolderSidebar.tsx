import { useState } from 'react'
import { Folder, Loader2, ChevronLeft, ChevronRight } from 'lucide-react'
import { FolderTree } from './FolderTree'
import { cn } from '@/lib/utils/cn'
import type { ConnectionFolderNode } from '@/types/connections'

interface FolderSidebarProps {
  folders: ConnectionFolderNode[]
  activeFolderId: string | null
  isLoading?: boolean
  onFolderSelect: (folderId: string | null) => void
}

export function FolderSidebar({
  folders,
  activeFolderId,
  isLoading,
  onFolderSelect,
}: FolderSidebarProps) {
  const [collapsed, setCollapsed] = useState(false)

  if (folders.length === 0) {
    return null
  }

  return (
    <div
      className={cn(
        'shrink-0 transition-all duration-300 ease-in-out',
        collapsed ? 'w-16' : 'w-72'
      )}
    >
      <div className="h-full overflow-hidden rounded-lg border border-border/60 bg-card shadow-sm">
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
        <div className={cn('overflow-y-auto', collapsed ? 'p-2' : 'p-3')}>
          {collapsed ? (
            // Collapsed: Show icon buttons
            <div className="space-y-2">
              <button
                onClick={() => onFolderSelect(null)}
                className={cn(
                  'flex h-10 w-full items-center justify-center rounded-md transition-colors',
                  !activeFolderId
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-muted'
                )}
                title="All Folders"
              >
                <Folder className="h-4 w-4" />
              </button>
              {folders.map((node) => (
                <FolderIconButton
                  key={node.folder.id}
                  node={node}
                  activeFolderId={activeFolderId}
                  onSelect={onFolderSelect}
                />
              ))}
            </div>
          ) : (
            // Expanded: Show full tree
            <>
              {/* "All Folders" button */}
              <button
                onClick={() => onFolderSelect(null)}
                className={cn(
                  'mb-2 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
                  !activeFolderId && !activeFolderId
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
              />
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// Recursive icon button for collapsed mode
function FolderIconButton({
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
      <button
        onClick={() => onSelect(node.folder.id)}
        className={cn(
          'flex h-10 w-full items-center justify-center rounded-md transition-colors',
          isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted'
        )}
        title={node.folder.name}
      >
        <Folder className="h-4 w-4" />
      </button>
      {node.children?.map((child) => (
        <FolderIconButton
          key={child.folder.id}
          node={child}
          activeFolderId={activeFolderId}
          onSelect={onSelect}
        />
      ))}
    </>
  )
}
