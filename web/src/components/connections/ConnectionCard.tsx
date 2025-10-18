import { MoreVertical, Pencil, Trash2, Share2, Rocket, Circle } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import type { Protocol } from '@/types/protocols'
import type { ActiveConnectionSession, ConnectionRecord } from '@/types/connections'
import { resolveConnectionIcon } from '@/constants/connections'
import { useLaunchConnectionContext } from '@/contexts/LaunchConnectionContext'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/DropdownMenu'

interface ConnectionCardProps {
  connection: ConnectionRecord
  protocol?: Protocol
  protocolIcon: LucideIcon
  teamName?: string
  onEdit?: (id: string) => void
  onDelete?: (id: string) => void
  onShare?: (id: string) => void
  activeSessions?: ActiveConnectionSession[]
  showActiveUsers?: boolean
}

export function ConnectionCard({
  connection,
  protocol,
  protocolIcon: ProtocolIcon,
  teamName,
  onEdit,
  onDelete,
  onShare,
  activeSessions,
}: ConnectionCardProps) {
  const launchContext = useLaunchConnectionContext()
  const metadata = connection.metadata ?? {}
  const endpoint = resolvePrimaryEndpoint(connection.targets, connection.settings)
  const isPersonal = !connection.team_id
  const metadataIcon = typeof metadata.icon === 'string' ? metadata.icon : undefined
  const metadataColor = typeof metadata.color === 'string' ? metadata.color : undefined
  const VisualIcon = metadataIcon ? resolveConnectionIcon(metadataIcon) : ProtocolIcon

  const activeSessionsList = activeSessions ?? []
  const sessionCount = activeSessionsList.length
  const hasActiveSessions = sessionCount > 0

  const hasActions = Boolean(onEdit || onShare || onDelete)

  return (
    <div className="group relative flex flex-col overflow-hidden rounded-xl border border-border/60 bg-card transition-all hover:border-border hover:shadow-md">
      {/* Card Content */}
      <div className="flex flex-1 flex-col p-5">
        {/* Header: Icon + Name + Actions */}
        <div className="mb-4 flex items-start gap-3">
          <div
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-primary/10 ring-1 ring-primary/15"
            style={
              metadataColor
                ? {
                    color: metadataColor,
                    backgroundColor: hexToRgba(metadataColor, 0.1),
                    boxShadow: `0 0 0 1px ${hexToRgba(metadataColor, 0.15)}`,
                  }
                : undefined
            }
          >
            <VisualIcon className="h-5 w-5" />
          </div>

          <div className="min-w-0 flex-1">
            <h3 className="truncate font-semibold text-foreground" title={connection.name}>
              {connection.name}
            </h3>
            <p
              className="mt-0.5 truncate text-sm text-muted-foreground"
              title={endpoint ?? 'Not configured'}
            >
              {endpoint ?? 'Not configured'}
            </p>
          </div>

          {hasActions && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0 opacity-0 transition-opacity group-hover:opacity-100"
                  aria-label="Actions"
                >
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {onEdit && (
                  <DropdownMenuItem onClick={() => onEdit(connection.id)}>
                    <Pencil className="mr-2 h-4 w-4" />
                    Edit
                  </DropdownMenuItem>
                )}
                {onShare && (
                  <DropdownMenuItem onClick={() => onShare(connection.id)}>
                    <Share2 className="mr-2 h-4 w-4" />
                    Share
                  </DropdownMenuItem>
                )}
                {onDelete && (
                  <>
                    {(onEdit || onShare) && <DropdownMenuSeparator />}
                    <DropdownMenuItem
                      onClick={() => onDelete(connection.id)}
                      className="text-destructive focus:text-destructive"
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      Delete
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>

        {/* Metadata: Protocol + Team + Live Status */}
        <div className="mb-4 flex flex-wrap items-center gap-2 text-xs">
          {protocol && <span className="text-muted-foreground">{protocol.name}</span>}
          {protocol && <span className="text-muted-foreground">•</span>}
          <span className="text-muted-foreground">
            {isPersonal ? 'Personal' : (teamName ?? 'Team')}
          </span>
          {hasActiveSessions && (
            <>
              <span className="text-muted-foreground">•</span>
              <span className="flex items-center gap-1 text-emerald-600">
                <Circle className="h-2 w-2 fill-current animate-pulse" />
                Live {sessionCount > 1 && `(${sessionCount})`}
              </span>
            </>
          )}
        </div>

        {/* Description (optional) */}
        {connection.description && (
          <p className="mb-4 line-clamp-2 text-sm text-muted-foreground">
            {connection.description}
          </p>
        )}
      </div>

      {/* Footer: Launch Button */}
      <div className="border-t border-border/40 bg-muted/10 p-3">
        <Button
          size="sm"
          className="w-full font-medium"
          onClick={() => launchContext.open(connection)}
        >
          <Rocket className="mr-1.5 h-4 w-4" />
          Connect
        </Button>
      </div>
    </div>
  )
}

function hexToRgba(hex: string, alpha: number) {
  const normalized = hex.trim().replace('#', '')
  if (normalized.length !== 6) {
    return hex
  }
  const r = Number.parseInt(normalized.slice(0, 2), 16)
  const g = Number.parseInt(normalized.slice(2, 4), 16)
  const b = Number.parseInt(normalized.slice(4, 6), 16)
  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) {
    return hex
  }
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

function resolvePrimaryEndpoint(
  targets?: Array<{ host: string; port?: number }>,
  settings?: Record<string, unknown>
) {
  if (targets && targets.length > 0) {
    const target = targets[0]
    if (target.host) {
      return target.port ? `${target.host}:${target.port}` : target.host
    }
  }
  const host = typeof settings?.host === 'string' ? settings.host : undefined
  const portValue = typeof settings?.port === 'number' ? settings.port : undefined
  return host ? (portValue ? `${host}:${portValue}` : host) : undefined
}
