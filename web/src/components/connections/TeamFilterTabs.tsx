import { useMemo } from 'react'
import { Filter } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { cn } from '@/lib/utils/cn'
import type { ConnectionRecord } from '@/types/connections'

interface Team {
  id: string
  name: string
}

interface TeamFilterTabsProps {
  teams: Team[]
  connections: ConnectionRecord[]
  activeTeam: string
  onTeamChange: (teamId: string) => void
}

export function TeamFilterTabs({
  teams,
  connections,
  activeTeam,
  onTeamChange,
}: TeamFilterTabsProps) {
  const teamCounts = useMemo(() => {
    const counts: Record<string, number> = {
      all: connections.length,
      personal: connections.filter((c) => !c.team_id).length,
      shared: connections.filter((c) => c.share_summary?.shared).length,
    }

    teams.forEach((team) => {
      counts[team.id] = connections.filter((c) => c.team_id === team.id).length
    })

    return counts
  }, [connections, teams])

  return (
    <div className="flex items-center gap-3 overflow-x-auto pb-2 scrollbar-thin">
      <Filter className="h-4 w-4 shrink-0 text-muted-foreground" />
      <div className="flex gap-2">
        <button
          onClick={() => onTeamChange('all')}
          className={cn(
            'flex shrink-0 items-center gap-2 whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium transition-all',
            activeTeam === 'all'
              ? 'bg-primary text-primary-foreground shadow-md ring-2 ring-primary/20'
              : 'bg-card text-muted-foreground shadow-sm ring-1 ring-border/60 hover:bg-accent hover:text-foreground hover:shadow'
          )}
        >
          All Connections
          <Badge
            variant={activeTeam === 'all' ? 'secondary' : 'outline'}
            className={cn(
              'text-xs font-semibold',
              activeTeam === 'all' && 'bg-primary-foreground/20'
            )}
          >
            {teamCounts.all}
          </Badge>
        </button>
        <button
          onClick={() => onTeamChange('personal')}
          className={cn(
            'flex shrink-0 items-center gap-2 whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium transition-all',
            activeTeam === 'personal'
              ? 'bg-primary text-primary-foreground shadow-md ring-2 ring-primary/20'
              : 'bg-card text-muted-foreground shadow-sm ring-1 ring-border/60 hover:bg-accent hover:text-foreground hover:shadow'
          )}
        >
          Personal
          <Badge
            variant={activeTeam === 'personal' ? 'secondary' : 'outline'}
            className={cn(
              'text-xs font-semibold',
              activeTeam === 'personal' && 'bg-primary-foreground/20'
            )}
          >
            {teamCounts.personal}
          </Badge>
        </button>
        <button
          onClick={() => onTeamChange('shared')}
          className={cn(
            'flex shrink-0 items-center gap-2 whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium transition-all',
            activeTeam === 'shared'
              ? 'bg-primary text-primary-foreground shadow-md ring-2 ring-primary/20'
              : 'bg-card text-muted-foreground shadow-sm ring-1 ring-border/60 hover:bg-accent hover:text-foreground hover:shadow'
          )}
        >
          Shared with me
          <Badge
            variant={activeTeam === 'shared' ? 'secondary' : 'outline'}
            className={cn(
              'text-xs font-semibold',
              activeTeam === 'shared' && 'bg-primary-foreground/20'
            )}
          >
            {teamCounts.shared}
          </Badge>
        </button>
        {teams.map((team) => (
          <button
            key={team.id}
            onClick={() => onTeamChange(team.id)}
            className={cn(
              'flex shrink-0 items-center gap-2 whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium transition-all',
              activeTeam === team.id
                ? 'bg-primary text-primary-foreground shadow-md ring-2 ring-primary/20'
                : 'bg-card text-muted-foreground shadow-sm ring-1 ring-border/60 hover:bg-accent hover:text-foreground hover:shadow'
            )}
          >
            {team.name}
            <Badge
              variant={activeTeam === team.id ? 'secondary' : 'outline'}
              className={cn(
                'text-xs font-semibold',
                activeTeam === team.id && 'bg-primary-foreground/20'
              )}
            >
              {teamCounts[team.id] || 0}
            </Badge>
          </button>
        ))}
      </div>
    </div>
  )
}
