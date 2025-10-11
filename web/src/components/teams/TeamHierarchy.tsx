import type { ReactElement } from 'react'
import type { TeamRecord } from '@/types/teams'
import { cn } from '@/lib/utils/cn'
import { Badge } from '@/components/ui/Badge'
import { FolderTree } from 'lucide-react'

interface TeamHierarchyProps {
  teams: TeamRecord[]
  selectedTeamId?: string
  memberCounts?: Record<string, number | undefined>
  onSelectTeam?: (teamId: string) => void
}

interface TreeNode {
  key: string
  label: string
  team?: TeamRecord
  children: Map<string, TreeNode>
}

function buildHierarchy(teams: TeamRecord[]): TreeNode[] {
  const root: TreeNode = {
    key: 'root',
    label: 'root',
    children: new Map<string, TreeNode>(),
  }

  teams.forEach((team) => {
    const segments = team.name
      .split('/')
      .map((part) => part.trim())
      .filter(Boolean)
    if (!segments.length) {
      return
    }

    let current = root

    segments.forEach((segment, index) => {
      const branchKey = segments.slice(0, index + 1).join('/')
      if (!current.children.has(segment)) {
        current.children.set(segment, {
          key: branchKey,
          label: segment,
          children: new Map<string, TreeNode>(),
        })
      }
      const node = current.children.get(segment)!
      if (index === segments.length - 1) {
        node.team = team
      }
      current = node
    })
  })

  return Array.from(root.children.values())
}

function renderNode(
  node: TreeNode,
  options: {
    depth: number
    selectedTeamId?: string
    memberCounts?: Record<string, number | undefined>
    onSelectTeam?: (teamId: string) => void
  }
): ReactElement {
  const { depth, selectedTeamId, memberCounts, onSelectTeam } = options
  const isSelectable = Boolean(node.team && onSelectTeam)
  const memberCount =
    (node.team?.id && memberCounts?.[node.team.id]) ?? node.team?.members?.length ?? undefined

  return (
    <li key={node.key} className={cn('py-1', depth > 0 && 'pl-4')}>
      <div className="flex items-center gap-2">
        <div
          className={cn(
            'h-px flex-1',
            depth === 0 ? 'bg-transparent' : 'bg-border',
            node.children.size > 0 && 'max-w-3'
          )}
        />
        {isSelectable ? (
          <button
            type="button"
            onClick={() => node.team && onSelectTeam?.(node.team.id)}
            className={cn(
              'flex min-w-0 flex-1 items-center justify-between rounded-md px-2 py-1 text-left text-sm transition',
              'hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              node.team?.id === selectedTeamId && 'bg-primary/10 text-primary'
            )}
          >
            <span className="truncate font-medium">{node.team?.name ?? node.label}</span>
            {typeof memberCount === 'number' ? (
              <Badge variant="outline" className="ml-3 shrink-0 text-[10px] font-medium">
                {memberCount} member{memberCount === 1 ? '' : 's'}
              </Badge>
            ) : null}
          </button>
        ) : (
          <div className="flex min-w-0 flex-1 items-center justify-between rounded-md px-2 py-1 text-sm text-muted-foreground">
            <span className="truncate">{node.label}</span>
            {typeof memberCount === 'number' ? (
              <Badge variant="outline" className="ml-3 shrink-0 text-[10px] font-medium">
                {memberCount} member{memberCount === 1 ? '' : 's'}
              </Badge>
            ) : null}
          </div>
        )}
      </div>
      {node.children.size ? (
        <ul className="ml-3 border-l border-dashed border-border pl-4">
          {Array.from(node.children.values()).map((child) =>
            renderNode(child, {
              depth: depth + 1,
              selectedTeamId,
              memberCounts,
              onSelectTeam,
            })
          )}
        </ul>
      ) : null}
    </li>
  )
}

export function TeamHierarchy({
  teams,
  selectedTeamId,
  memberCounts,
  onSelectTeam,
}: TeamHierarchyProps) {
  const nodes = buildHierarchy(teams)

  if (!nodes.length) {
    return null
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
          <FolderTree className="h-5 w-5 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-semibold text-foreground">Team hierarchy</p>
          <p className="text-xs text-muted-foreground">
            Derived from “/” separated segments in team names
          </p>
        </div>
      </div>

      <ul className="space-y-1 text-sm">
        {nodes.map((node) =>
          renderNode(node, {
            depth: 0,
            selectedTeamId,
            memberCounts,
            onSelectTeam,
          })
        )}
      </ul>
    </div>
  )
}
