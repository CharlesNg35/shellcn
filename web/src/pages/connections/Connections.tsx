import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'
import {
  Cloud,
  Container,
  Database,
  Filter,
  Folder,
  HardDrive,
  Loader2,
  Monitor,
  MoreVertical,
  Network,
  Plus,
  Search,
  Server,
} from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { useAvailableProtocols } from '@/hooks/useProtocols'
import { useConnections } from '@/hooks/useConnections'
import type { Protocol } from '@/types/protocols'
import type { ConnectionRecord, ConnectionTarget } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

const CATEGORY_ICON_MAP: Record<string, LucideIcon> = {
  terminal: Server,
  desktop: Monitor,
  container: Container,
  database: Database,
  file_share: Folder,
  vm: HardDrive,
  network: Network,
}

const DEFAULT_PROTOCOL_ICON = Server

interface ProtocolTab {
  id: string
  label: string
  icon: LucideIcon
  count: number
  features: string[]
}

export function Connections() {
  const [selectedTab, setSelectedTab] = useState<string>('all')
  const [search, setSearch] = useState('')

  const {
    data: protocols = [],
    isLoading: protocolsLoading,
    isError: protocolsError,
  } = useAvailableProtocols()
  const {
    data: connectionsResult,
    isLoading: connectionsLoading,
    isError: connectionsError,
  } = useConnections()
  const connections = connectionsResult?.data ?? []

  const protocolLookup = useMemo(() => {
    return protocols.reduce<Record<string, Protocol>>((acc, protocol) => {
      acc[protocol.id] = protocol
      return acc
    }, {})
  }, [protocols])

  const normalizedSearch = search.trim().toLowerCase()

  const filteredConnections = useMemo(() => {
    return connections.filter((connection) => {
      const matchesProtocol = selectedTab === 'all' || connection.protocol_id === selectedTab
      if (!matchesProtocol) {
        return false
      }

      if (!normalizedSearch) {
        return true
      }

      const protocol = protocolLookup[connection.protocol_id]
      const metadata = connection.metadata ?? {}
      const targets = connection.targets ?? []
      const tags = extractTags(metadata)
      const hostMatches = targets.some((target) =>
        target.host.toLowerCase().includes(normalizedSearch)
      )
      const metadataMatch = Object.values(metadata).some(
        (value) => typeof value === 'string' && value.toLowerCase().includes(normalizedSearch)
      )
      const tagMatch = tags.some((tag) => tag.toLowerCase().includes(normalizedSearch))

      return (
        connection.name.toLowerCase().includes(normalizedSearch) ||
        (connection.description?.toLowerCase().includes(normalizedSearch) ?? false) ||
        hostMatches ||
        metadataMatch ||
        tagMatch ||
        protocol?.name.toLowerCase().includes(normalizedSearch)
      )
    })
  }, [connections, normalizedSearch, protocolLookup, selectedTab])

  const tabs: ProtocolTab[] = useMemo(() => {
    const counts = connections.reduce<Record<string, number>>((acc, connection) => {
      acc[connection.protocol_id] = (acc[connection.protocol_id] ?? 0) + 1
      return acc
    }, {})

    const base: ProtocolTab[] = protocols.map((protocol) => ({
      id: protocol.id,
      label: protocol.name,
      icon: resolveProtocolIcon(protocol),
      count: counts[protocol.id] ?? 0,
      features: protocol.features,
    }))

    return [
      {
        id: 'all',
        label: 'All Connections',
        icon: DEFAULT_PROTOCOL_ICON,
        count: connections.length,
        features: [],
      },
      ...base,
    ]
  }, [connections, protocols])

  const isLoading = protocolsLoading || connectionsLoading
  const hasError = protocolsError || connectionsError

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
          <p className="text-sm text-muted-foreground">
            Discover available protocol drivers and launch saved infrastructure connections
          </p>
        </div>
        <Button asChild size="sm">
          <Link to="/connections/new">
            <Plus className="mr-2 h-4 w-4" />
            New Connection
          </Link>
        </Button>
      </header>

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by name, host, or tag"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            className="pl-9"
          />
        </div>
        <Button variant="outline" size="sm">
          <Filter className="mr-2 h-4 w-4" />
          Filter
        </Button>
      </div>

      <ProtocolTabs
        tabs={tabs}
        isLoading={protocolsLoading}
        activeTab={selectedTab}
        onTabChange={setSelectedTab}
      />

      {isLoading ? (
        <LoadingState />
      ) : hasError ? (
        <ErrorState />
      ) : filteredConnections.length === 0 ? (
        <EmptyState hasProtocols={protocols.length > 0} search={normalizedSearch} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filteredConnections.map((connection) => (
            <ConnectionCard
              key={connection.id}
              connection={connection}
              protocol={protocolLookup[connection.protocol_id]}
            />
          ))}
        </div>
      )}
    </div>
  )
}

interface ProtocolTabsProps {
  tabs: ProtocolTab[]
  isLoading: boolean
  activeTab: string
  onTabChange: (tabId: string) => void
}

function ProtocolTabs({ tabs, isLoading, activeTab, onTabChange }: ProtocolTabsProps) {
  if (isLoading && !tabs.length) {
    return (
      <div className="flex gap-2 overflow-x-auto pb-2">
        {Array.from({ length: 4 }).map((_, index) => (
          <div
            key={`protocol-skeleton-${index}`}
            className="h-10 w-32 animate-pulse rounded-md bg-muted"
          />
        ))}
      </div>
    )
  }

  return (
    <div className="flex gap-2 overflow-x-auto pb-2">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => onTabChange(tab.id)}
          className={cn(
            'flex items-center gap-2 whitespace-nowrap rounded-md px-4 py-2 text-sm font-medium transition-colors',
            activeTab === tab.id
              ? 'bg-primary text-primary-foreground shadow'
              : 'bg-muted text-muted-foreground hover:bg-accent hover:text-accent-foreground'
          )}
        >
          <tab.icon className="h-4 w-4" />
          <span>{tab.label}</span>
          <Badge variant="secondary" className="ml-1">
            {tab.count}
          </Badge>
        </button>
      ))}
    </div>
  )
}

interface ConnectionCardProps {
  connection: ConnectionRecord
  protocol?: Protocol
}

function ConnectionCard({ connection, protocol }: ConnectionCardProps) {
  const tags = extractTags(connection.metadata)
  const endpoint = resolvePrimaryEndpoint(connection.targets, connection.settings)
  const status = resolveStatus(connection)
  const ProtocolIcon = resolveProtocolIcon(protocol)

  return (
    <div className="group relative rounded-lg border border-border bg-card p-4 shadow-sm transition-shadow hover:shadow-md">
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10">
            <ProtocolIcon className="h-5 w-5 text-primary" />
          </div>
          <div>
            <h3 className="font-semibold">{connection.name}</h3>
            <p className="text-sm text-muted-foreground">{endpoint ?? 'No target configured'}</p>
            {protocol ? (
              <p className="text-xs uppercase tracking-wide text-muted-foreground">
                {protocol.name}
              </p>
            ) : null}
          </div>
        </div>
        <button className="rounded-md p-1 opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100">
          <MoreVertical className="h-4 w-4" />
        </button>
      </div>

      <div className="mt-3 flex items-center gap-2">
        <StatusDot status={status} />
        <span className="text-xs capitalize text-muted-foreground">{status}</span>
        {connection.last_used_at ? (
          <span className="text-xs text-muted-foreground">
            Last used {new Date(connection.last_used_at).toLocaleDateString()}
          </span>
        ) : null}
      </div>

      {tags.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1">
          {tags.map((tag) => (
            <Badge key={tag} variant="outline" className="text-xs">
              {tag}
            </Badge>
          ))}
        </div>
      )}

      {protocol?.features?.length ? (
        <div className="mt-4 flex flex-wrap gap-1">
          {protocol.features.map((feature) => (
            <Badge key={feature} variant="secondary" className="text-xs uppercase">
              {feature.replace(/_/g, ' ')}
            </Badge>
          ))}
        </div>
      ) : null}

      <div className="mt-4 flex gap-2">
        <Button size="sm" className="flex-1" asChild>
          <Link to={`/connections/${connection.id}`}>Launch</Link>
        </Button>
        <Button size="sm" variant="outline" asChild>
          <Link to={`/connections/${connection.id}/edit`}>Edit</Link>
        </Button>
      </div>
    </div>
  )
}

interface EmptyStateProps {
  hasProtocols: boolean
  search: string
}

function EmptyState({ hasProtocols, search }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/50 py-16 text-center">
      <Server className="mb-4 h-12 w-12 text-muted-foreground" />
      <h3 className="mb-2 text-lg font-semibold">{search ? 'No matches' : 'No connections yet'}</h3>
      <p className="mb-4 max-w-md text-sm text-muted-foreground">
        {search
          ? 'Try refining your search or switch to a different protocol tab.'
          : hasProtocols
            ? 'Create a connection to reuse driver settings and shared identities.'
            : 'No protocol drivers are currently available. Check your permissions or driver health.'}
      </p>
      {!search && (
        <Button asChild size="sm">
          <Link to="/connections/new">
            <Plus className="mr-2 h-4 w-4" />
            Create Connection
          </Link>
        </Button>
      )}
    </div>
  )
}

function LoadingState() {
  return (
    <div className="flex min-h-[200px] items-center justify-center rounded-lg border border-dashed border-border">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        <span>Loading connections…</span>
      </div>
    </div>
  )
}

function ErrorState() {
  return (
    <div className="flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-6 text-center">
      <p className="font-semibold text-destructive">Unable to load connections</p>
      <p className="text-sm text-destructive">
        Check your network connection or verify you have the required permissions.
      </p>
    </div>
  )
}

function resolveProtocolIcon(protocol?: Protocol): LucideIcon {
  if (protocol?.icon) {
    const iconKey = protocol.icon.toLowerCase()
    switch (iconKey) {
      case 'server':
        return Server
      case 'monitor':
        return Monitor
      case 'database':
        return Database
      case 'container':
        return Container
      case 'cloud':
        return Cloud
      case 'harddrive':
      case 'hard_drive':
        return HardDrive
      case 'folder':
      case 'files':
        return Folder
      default:
        break
    }
  }

  if (protocol?.category) {
    const icon = CATEGORY_ICON_MAP[protocol.category.toLowerCase()]
    if (icon) {
      return icon
    }
  }

  return DEFAULT_PROTOCOL_ICON
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

function resolvePrimaryEndpoint(targets?: ConnectionTarget[], settings?: Record<string, unknown>) {
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

function StatusDot({ status }: { status: string }) {
  const color =
    status === 'connected'
      ? 'bg-green-500'
      : status === 'error'
        ? 'bg-destructive'
        : status === 'ready'
          ? 'bg-blue-500'
          : 'bg-muted-foreground'
  return <span className={cn('h-2 w-2 rounded-full', color)} />
}
