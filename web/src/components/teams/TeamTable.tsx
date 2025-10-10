import type { ReactNode } from 'react'
import { PencilLine, Trash2, UserPlus, Users } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import type { TeamRecord } from '@/types/teams'
import { cn } from '@/lib/utils/cn'

interface TeamTableProps {
  teams: TeamRecord[]
  selectedTeamId?: string
  isLoading?: boolean
  memberCounts?: Record<string, number | undefined>
  onSelectTeam: (teamId: string) => void
  onEditTeam?: (team: TeamRecord) => void
  onDeleteTeam?: (team: TeamRecord) => void
  emptyAction?: ReactNode
}

export function TeamTable({
  teams,
  selectedTeamId,
  isLoading,
  memberCounts,
  onSelectTeam,
  onEditTeam,
  onDeleteTeam,
  emptyAction,
}: TeamTableProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="mb-4 flex items-center gap-2">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
            <Users className="h-5 w-5 text-muted-foreground" />
          </div>
          <div>
            <p className="text-lg font-semibold text-foreground">Teams</p>
            <p className="text-sm text-muted-foreground">Loading teams…</p>
          </div>
        </div>
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="rounded-lg border border-border/50 bg-muted/40 p-4">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="mt-2 h-3 w-48" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (!teams.length) {
    return (
      <EmptyState
        icon={UserPlus}
        title="No teams created yet"
        description="Organize your users into teams to simplify permission management and access control."
        action={emptyAction}
      />
    )
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
            <Users className="h-5 w-5 text-muted-foreground" />
          </div>
          <div>
            <p className="text-lg font-semibold text-foreground">Teams</p>
            <p className="text-sm text-muted-foreground">
              Select a team to view details and manage membership
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-2">
        {teams.map((team) => {
          const isSelected = team.id === selectedTeamId
          const memberCount = memberCounts?.[team.id] ?? team.members?.length

          return (
            <button
              key={team.id}
              type="button"
              onClick={() => onSelectTeam(team.id)}
              className={cn(
                'group flex w-full items-center justify-between gap-4 rounded-lg border border-transparent px-4 py-3 text-left transition',
                'hover:border-border hover:bg-muted/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                isSelected && 'border-primary bg-primary/5'
              )}
            >
              <div className="flex flex-col">
                <span className="text-sm font-medium text-foreground">{team.name}</span>
                {team.description ? (
                  <span className="mt-1 text-xs text-muted-foreground line-clamp-2">
                    {team.description}
                  </span>
                ) : (
                  <span className="mt-1 text-xs text-muted-foreground">
                    No description provided
                  </span>
                )}
              </div>

              <div className="flex items-center gap-3">
                <Badge variant="secondary" className="flex items-center gap-1 text-xs font-medium">
                  <Users className="h-3 w-3" />
                  {typeof memberCount === 'number' ? `${memberCount} members` : '—'}
                </Badge>

                <div className="hidden gap-1 group-hover:flex">
                  <PermissionGuard permission="team.manage">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      aria-label={`Edit ${team.name}`}
                      onClick={(event) => {
                        event.stopPropagation()
                        onEditTeam?.(team)
                      }}
                    >
                      <PencilLine className="h-4 w-4" />
                    </Button>
                  </PermissionGuard>
                  <PermissionGuard permission="team.manage">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:text-destructive"
                      aria-label={`Delete ${team.name}`}
                      onClick={(event) => {
                        event.stopPropagation()
                        onDeleteTeam?.(team)
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </PermissionGuard>
                </div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
