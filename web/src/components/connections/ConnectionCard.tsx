import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { MoreVertical, Clock, Pencil, Trash2, Share2, Rocket } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import type { Protocol } from '@/types/protocols'
import type { ActiveConnectionSession, ConnectionRecord } from '@/types/connections'
import { cn } from '@/lib/utils/cn'
import { useState } from 'react'
import { resolveConnectionIcon } from '@/constants/connections'
import { useLaunchConnectionContext } from '@/contexts/LaunchConnectionContext'

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
  showActiveUsers = false,
}: ConnectionCardProps) {
  const [showMenu, setShowMenu] = useState(false)
  const launchContext = useLaunchConnectionContext()
  const metadata = connection.metadata ?? {}
  const tags = extractTags(metadata)
  const endpoint = resolvePrimaryEndpoint(connection.targets, connection.settings)
  const status = resolveStatus(connection)
  const isPersonal = !connection.team_id
  const metadataIcon = typeof metadata.icon === 'string' ? metadata.icon : undefined
  const metadataColor = typeof metadata.color === 'string' ? metadata.color : undefined
  const VisualIcon = metadataIcon ? resolveConnectionIcon(metadataIcon) : ProtocolIcon
  const iconAccentStyle = metadataColor
    ? {
        color: metadataColor,
        backgroundColor: hexToRgba(metadataColor, 0.12),
        boxShadow: `0 0 0 1px ${hexToRgba(metadataColor, 0.2)}`,
      }
    : undefined

  const shareSummary = connection.share_summary
  const shareEntries = shareSummary?.entries ?? []
  const hasShare = Boolean(shareSummary?.shared && shareEntries.length > 0)

  const shareBadgeLabel = hasShare
    ? (() => {
        const primary = shareEntries[0]
        const granterName = primary.granted_by?.name ?? 'Shared'
        const baseLabel = primary.granted_by ? `Shared by ${granterName}` : 'Shared access'
        if (primary.principal.type === 'team') {
          return `${baseLabel} • ${primary.principal.name}`
        }
        return baseLabel
      })()
    : null

  const shareTooltip = hasShare
    ? shareEntries
        .map((entry) => {
          const granter = entry.granted_by?.name
            ? `Shared by ${entry.granted_by.name}`
            : 'Shared access'
          const via =
            entry.principal.type === 'team' ? `Team: ${entry.principal.name}` : 'Direct access'
          const scopes = entry.permission_scopes.length
            ? entry.permission_scopes.join(', ')
            : 'connection.view'
          const expiry = entry.expires_at
            ? `Expires ${formatDistanceToNow(new Date(entry.expires_at), { addSuffix: true })}`
            : 'No expiry'
          return `${granter} • ${via} • ${scopes} • ${expiry}`
        })
        .join('\n')
    : undefined

  const activeSessionsList = activeSessions ?? []
  const sessionCount = activeSessionsList.length
  const hasActiveSessions = sessionCount > 0

  const formatRelative = (value?: string) => {
    if (!value) {
      return null
    }
    const timestamp = new Date(value)
    if (Number.isNaN(timestamp.getTime())) {
      return null
    }
    return formatDistanceToNow(timestamp, { addSuffix: true })
  }

  const latestSessionLabel = hasActiveSessions
    ? formatRelative(activeSessionsList[0]?.last_seen_at ?? undefined)
    : null

  const activeBadgeTitle = hasActiveSessions
    ? showActiveUsers
      ? activeSessionsList
          .map((session) => {
            const userName = session.user_name?.trim() || session.user_id
            const since = formatRelative(session.started_at)
            return since ? `${userName} • ${since}` : userName
          })
          .join('\n')
      : latestSessionLabel
        ? `Last activity ${latestSessionLabel}`
        : undefined
    : undefined

  const recordingEnabled =
    connection.protocol_id === 'ssh' && Boolean(connection.settings?.recording_enabled)

  return (
    <div className="group relative flex flex-col overflow-hidden rounded-lg border border-border/60 bg-card transition-all hover:border-border hover:shadow-lg">
      {hasActiveSessions && (
        <Badge
          variant="success"
          className="absolute right-3 top-3 z-10 flex items-center gap-1 text-[11px] font-semibold"
          title={activeBadgeTitle}
        >
          <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
          <span>Live{sessionCount > 1 ? ` (${sessionCount})` : ''}</span>
        </Badge>
      )}
      {/* Card Header */}
      <div className="flex items-start justify-between border-b border-border/40 bg-muted/30 p-4">
        <div className="flex items-start gap-3 flex-1 min-w-0">
          <div
            className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-primary/10 ring-1 ring-primary/20"
            style={iconAccentStyle}
          >
            <VisualIcon className="h-6 w-6" />
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="font-semibold text-base truncate" title={connection.name}>
              {connection.name}
            </h3>
            <p
              className="text-sm text-muted-foreground truncate mt-0.5"
              title={endpoint ?? 'No target configured'}
            >
              {endpoint ?? 'No target configured'}
            </p>
          </div>
        </div>

        {/* Actions Menu */}
        <div className="relative">
          <button
            onClick={() => setShowMenu(!showMenu)}
            className="rounded-md p-1.5 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground group-hover:opacity-100"
            aria-label="Connection actions"
          >
            <MoreVertical className="h-4 w-4" />
          </button>

          {showMenu && (
            <>
              <div className="fixed inset-0 z-10" onClick={() => setShowMenu(false)} />
              <div className="absolute right-0 top-8 z-20 w-48 rounded-md border border-border bg-popover p-1 shadow-lg">
                <button
                  onClick={() => {
                    onEdit?.(connection.id)
                    setShowMenu(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-3 py-2 text-sm text-foreground hover:bg-accent"
                >
                  <Pencil className="h-4 w-4" />
                  Edit
                </button>
                {onShare && (
                  <button
                    onClick={() => {
                      onShare(connection.id)
                      setShowMenu(false)
                    }}
                    className="flex w-full items-center gap-2 rounded-sm px-3 py-2 text-sm text-foreground hover:bg-accent"
                  >
                    <Share2 className="h-4 w-4" />
                    Share
                  </button>
                )}
                <button
                  onClick={() => {
                    onDelete?.(connection.id)
                    setShowMenu(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
                >
                  <Trash2 className="h-4 w-4" />
                  Delete
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Card Body */}
      <div className="flex flex-1 flex-col p-4 space-y-3">
        {/* Badges */}
        <div className="flex flex-wrap items-center gap-2">
          {protocol && (
            <Badge variant="outline" className="text-xs font-medium">
              {protocol.name}
            </Badge>
          )}
          <Badge variant={isPersonal ? 'secondary' : 'default'} className="text-xs font-medium">
            {isPersonal ? 'Personal' : (teamName ?? 'Team')}
          </Badge>
          {recordingEnabled ? (
            <Badge variant="destructive" className="text-xs font-medium">
              Recording enabled
            </Badge>
          ) : null}
          {hasShare && shareBadgeLabel && (
            <Badge variant="secondary" className="text-xs font-medium" title={shareTooltip}>
              {shareBadgeLabel}
            </Badge>
          )}
        </div>

        {/* Status */}
        <div className="flex items-center gap-2">
          <StatusDot status={status} />
          <span className="text-xs capitalize text-muted-foreground">{status}</span>
          {connection.last_used_at && (
            <>
              <span className="text-xs text-muted-foreground">•</span>
              <div className="flex items-center gap-1 text-xs text-muted-foreground">
                <Clock className="h-3 w-3" />
                <span>{formatLastUsed(connection.last_used_at)}</span>
              </div>
            </>
          )}
        </div>

        {/* Description */}
        {connection.description && (
          <p className="text-sm text-muted-foreground line-clamp-2">{connection.description}</p>
        )}

        {/* Tags */}
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {tags.slice(0, 3).map((tag) => (
              <Badge key={tag} variant="outline" className="text-xs">
                {tag}
              </Badge>
            ))}
            {tags.length > 3 && (
              <Badge variant="outline" className="text-xs">
                +{tags.length - 3}
              </Badge>
            )}
          </div>
        )}

        {/* Features */}
        {protocol?.features && protocol.features.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pt-2 border-t border-border/40">
            {protocol.features.slice(0, 3).map((feature) => (
              <Badge
                key={feature}
                variant="secondary"
                className="text-[10px] uppercase tracking-wide"
              >
                {feature.replace(/_/g, ' ')}
              </Badge>
            ))}
            {protocol.features.length > 3 && (
              <Badge variant="secondary" className="text-[10px]">
                +{protocol.features.length - 3}
              </Badge>
            )}
          </div>
        )}
      </div>

      {/* Card Footer */}
      <div className="border-t border-border/40 bg-muted/20 p-3">
        <div className="flex gap-2">
          <Button
            size="sm"
            className="flex-1 font-medium"
            onClick={() => launchContext.open(connection)}
          >
            <Rocket className="mr-1.5 h-3.5 w-3.5" />
            Launch
          </Button>
          <Button size="sm" variant="outline" asChild>
            <Link to={`/connections/${connection.id}/edit`}>
              <Pencil className="h-3.5 w-3.5" />
            </Link>
          </Button>
        </div>
      </div>
    </div>
  )
}

function extractTags(metadata?: Record<string, unknown>): string[] {
  if (!metadata) {
    return []
  }
  const raw = metadata.tags
  if (Array.isArray(raw)) {
    return raw.filter((tag): tag is string => typeof tag === 'string')
  }
  return []
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

function resolveStatus(connection: ConnectionRecord): string {
  const metadataStatus = connection.metadata?.status
  if (typeof metadataStatus === 'string') {
    return metadataStatus.toLowerCase()
  }
  return 'ready'
}

function formatLastUsed(lastUsedAt: string): string {
  const date = new Date(lastUsedAt)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return 'Today'
  }
  if (diffDays === 1) {
    return 'Yesterday'
  }
  if (diffDays < 7) {
    return `${diffDays}d ago`
  }
  if (diffDays < 30) {
    return `${Math.floor(diffDays / 7)}w ago`
  }
  return date.toLocaleDateString()
}

function StatusDot({ status }: { status: string }) {
  const color =
    status === 'connected'
      ? 'bg-green-500'
      : status === 'error'
        ? 'bg-destructive'
        : status === 'ready'
          ? 'bg-blue-500'
          : 'bg-muted-foreground'
  return (
    <span className="relative flex h-2.5 w-2.5">
      <span
        className={cn(
          'absolute inline-flex h-full w-full animate-ping rounded-full opacity-75',
          color,
          status !== 'connected' && 'hidden'
        )}
      />
      <span className={cn('relative inline-flex h-2.5 w-2.5 rounded-full', color)} />
    </span>
  )
}
