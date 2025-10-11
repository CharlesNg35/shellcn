import { useCallback, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, Folder as FolderIcon } from 'lucide-react'
import { Link, useInRouterContext } from 'react-router-dom'
import type { ConnectionFolderNode } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

interface FolderTreeProps {
  nodes?: ConnectionFolderNode[]
  activeFolderId?: string | null
  onSelect?: (folderId: string | null) => void
  basePath?: string
  className?: string
  disableNavigation?: boolean
  renderActions?: (folder: ConnectionFolderNode) => React.ReactNode
}

export function FolderTree({
  nodes,
  activeFolderId,
  onSelect,
  basePath = '/connections',
  className,
  disableNavigation = false,
  renderActions,
}: FolderTreeProps) {
  // Initialize all nodes as open by default
  const [openNodes, setOpenNodes] = useState<Record<string, boolean>>(() => {
    const initialState: Record<string, boolean> = {}
    const initializeNodes = (nodeList: ConnectionFolderNode[]) => {
      nodeList.forEach((node) => {
        initialState[node.folder.id] = true
        if (node.children) {
          initializeNodes(node.children)
        }
      })
    }
    if (nodes) {
      initializeNodes(nodes)
    }
    return initialState
  })

  const toggleNode = useCallback((nodeId: string) => {
    setOpenNodes((prev) => ({
      ...prev,
      [nodeId]: !prev[nodeId],
    }))
  }, [])

  const handleSelect = useCallback(
    (folderId: string | null) => {
      onSelect?.(folderId)
    },
    [onSelect]
  )

  const tree = useMemo(() => nodes ?? [], [nodes])

  if (!tree.length) {
    return (
      <div
        className={cn(
          'rounded-md border border-dashed border-border/60 px-3 py-4 text-center',
          className
        )}
      >
        <p className="text-xs text-muted-foreground">No folders yet</p>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col gap-1', className)}>
      {tree.map((node) => (
        <FolderTreeNode
          key={node.folder.id}
          node={node}
          openNodes={openNodes}
          onToggle={toggleNode}
          onSelect={handleSelect}
          basePath={basePath}
          depth={0}
          activeFolderId={activeFolderId}
          disableNavigation={disableNavigation}
          renderActions={renderActions}
        />
      ))}
    </div>
  )
}

interface FolderTreeNodeProps {
  node: ConnectionFolderNode
  openNodes: Record<string, boolean>
  onToggle: (id: string) => void
  onSelect: (folderId: string | null) => void
  basePath: string
  depth: number
  activeFolderId?: string | null
  disableNavigation?: boolean
  renderActions?: (folder: ConnectionFolderNode) => React.ReactNode
}

function FolderTreeNode({
  node,
  openNodes,
  onToggle,
  onSelect,
  basePath,
  depth,
  activeFolderId,
  disableNavigation,
  renderActions,
}: FolderTreeNodeProps) {
  const hasChildren = node.children && node.children.length > 0
  const isOpen = openNodes[node.folder.id] ?? true
  const paddingLeft = depth * 12
  const isFolderActive = activeFolderId === node.folder.id
  const href =
    node.folder.id === 'unassigned'
      ? `${basePath}?view=unassigned`
      : `${basePath}?folder=${node.folder.id}`
  const connectionBadge = node.connection_count ? (
    <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
      {node.connection_count}
    </span>
  ) : null

  return (
    <div>
      <div
        className={cn(
          'group flex items-center rounded-md px-2 py-1.5 text-sm transition hover:bg-muted',
          isFolderActive && 'bg-muted text-foreground'
        )}
        style={{ paddingLeft }}
      >
        {hasChildren ? (
          <button
            type="button"
            onClick={() => onToggle(node.folder.id)}
            className="mr-1 inline-flex h-5 w-5 items-center justify-center text-muted-foreground hover:text-foreground"
          >
            {isOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </button>
        ) : (
          <span className="mr-1 h-5 w-5" />
        )}

        <div className="flex flex-1 items-center gap-2">
          {useInRouterContext() && !disableNavigation ? (
            <Link
              to={href}
              className="flex flex-1 items-center gap-2 text-muted-foreground hover:text-foreground"
              onClick={() => onSelect(node.folder.id === 'unassigned' ? null : node.folder.id)}
            >
              <FolderIcon className="h-4 w-4" />
              <span className="truncate">{node.folder.name}</span>
            </Link>
          ) : (
            <button
              type="button"
              className="flex flex-1 items-center gap-2 text-left text-muted-foreground hover:text-foreground"
              onClick={() => onSelect(node.folder.id === 'unassigned' ? null : node.folder.id)}
            >
              <FolderIcon className="h-4 w-4" />
              <span className="truncate">{node.folder.name}</span>
            </button>
          )}

          {connectionBadge}

          {renderActions ? (
            <div className="ml-1 opacity-0 transition-opacity group-hover:opacity-100">
              {renderActions(node)}
            </div>
          ) : null}
        </div>
      </div>
      {hasChildren && isOpen && (
        <div className="space-y-1">
          {node.children!.map((child) => (
            <FolderTreeNode
              key={child.folder.id}
              node={child}
              openNodes={openNodes}
              onToggle={onToggle}
              onSelect={onSelect}
              basePath={basePath}
              depth={depth + 1}
              activeFolderId={activeFolderId}
              disableNavigation={disableNavigation}
            />
          ))}
        </div>
      )}
    </div>
  )
}
