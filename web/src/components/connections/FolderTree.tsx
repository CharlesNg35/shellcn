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
}

export function FolderTree({
  nodes,
  activeFolderId,
  onSelect,
  basePath = '/connections',
  className,
  disableNavigation = false,
}: FolderTreeProps) {
  const [openNodes, setOpenNodes] = useState<Record<string, boolean>>({})

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
          isOpen={openNodes[node.folder.id] ?? true}
          onToggle={toggleNode}
          onSelect={handleSelect}
          basePath={basePath}
          depth={0}
          activeFolderId={activeFolderId}
          disableNavigation={disableNavigation}
        />
      ))}
    </div>
  )
}

interface FolderTreeNodeProps {
  node: ConnectionFolderNode
  isOpen: boolean
  onToggle: (id: string) => void
  onSelect: (folderId: string | null) => void
  basePath: string
  depth: number
  activeFolderId?: string | null
  disableNavigation?: boolean
}

function FolderTreeNode({
  node,
  isOpen,
  onToggle,
  onSelect,
  basePath,
  depth,
  activeFolderId,
  disableNavigation,
}: FolderTreeNodeProps) {
  const hasChildren = node.children && node.children.length > 0
  const paddingLeft = depth * 12
  const isFolderActive = activeFolderId === node.folder.id
  const href =
    node.folder.id === 'unassigned'
      ? `${basePath}?view=unassigned`
      : `${basePath}?folder=${node.folder.id}`

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

        {useInRouterContext() && !disableNavigation ? (
          <Link
            to={href}
            className="flex flex-1 items-center gap-2 text-muted-foreground hover:text-foreground"
            onClick={() => onSelect(node.folder.id === 'unassigned' ? null : node.folder.id)}
          >
            <FolderIcon className="h-4 w-4" />
            <span className="truncate">{node.folder.name}</span>
            {node.connection_count ? (
              <span className="ml-auto rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {node.connection_count}
              </span>
            ) : null}
          </Link>
        ) : (
          <a
            href={href}
            className="flex flex-1 items-center gap-2 text-muted-foreground hover:text-foreground"
            onClick={(e) => {
              e.preventDefault()
              onSelect(node.folder.id === 'unassigned' ? null : node.folder.id)
            }}
          >
            <FolderIcon className="h-4 w-4" />
            <span className="truncate">{node.folder.name}</span>
            {node.connection_count ? (
              <span className="ml-auto rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {node.connection_count}
              </span>
            ) : null}
          </a>
        )}
      </div>
      {hasChildren && isOpen ? (
        <div className="mt-1 space-y-1">
          {node.children!.map((child) => (
            <FolderTreeNode
              key={child.folder.id}
              node={child}
              isOpen={true}
              onToggle={onToggle}
              onSelect={onSelect}
              basePath={basePath}
              depth={depth + 1}
              activeFolderId={activeFolderId}
            />
          ))}
        </div>
      ) : null}
    </div>
  )
}
