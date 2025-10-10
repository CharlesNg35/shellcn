import { useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'
import {
  Cloud,
  Container,
  Database,
  Folder,
  HardDrive,
  Monitor,
  Network,
  Plus,
  Search,
  Server,
  X,
} from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { useAvailableProtocols } from '@/hooks/useProtocols'
import { useConnections } from '@/hooks/useConnections'
import { useConnectionFolders } from '@/hooks/useConnectionFolders'
import { useTeams } from '@/hooks/useTeams'
import { usePermissions } from '@/hooks/usePermissions'
import type { Protocol } from '@/types/protocols'
import { ConnectionCard } from '@/components/connections/ConnectionCard'
import { TeamFilterTabs } from '@/components/connections/TeamFilterTabs'
import { FolderSidebar } from '@/components/connections/FolderSidebar'
import { cn } from '@/lib/utils/cn'
import { PERMISSIONS } from '@/constants/permissions'

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
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedProtocol, setSelectedProtocol] = useState<string | null>(null)
  const [search, setSearch] = useState(searchParams.get('search') ?? '')
  const activeFolder = searchParams.get('folder')
  const teamParam = searchParams.get('team') ?? 'all'

  const teamFilterValue =
    teamParam === 'all' ? undefined : teamParam === 'personal' ? 'personal' : teamParam

  const { hasPermission } = usePermissions()
  const canViewTeams = hasPermission(PERMISSIONS.TEAM.VIEW)
  const { data: teamsResult } = useTeams({
    enabled: canViewTeams,
    staleTime: 60_000,
  })
  const teams = useMemo(() => teamsResult?.data ?? [], [teamsResult?.data])
  const teamLookup = useMemo(() => {
    return teams.reduce<Record<string, string>>((acc, team) => {
      acc[team.id] = team.name
      return acc
    }, {})
  }, [teams])

  const {
    data: protocolsResult,
    isLoading: protocolsLoading,
    isError: protocolsError,
  } = useAvailableProtocols()
  const protocols = useMemo(() => protocolsResult?.data ?? [], [protocolsResult])
  const { data: folderTree = [], isLoading: foldersLoading } = useConnectionFolders(
    teamFilterValue,
    {
      staleTime: 60_000,
    }
  )

  const {
    data: connectionsResult,
    isLoading: connectionsLoading,
    isError: connectionsError,
  } = useConnections({
    folder_id: activeFolder || undefined,
    search: search || undefined,
    team_id: teamFilterValue,
  })
  const connections = useMemo(() => connectionsResult?.data ?? [], [connectionsResult?.data])

  const protocolLookup = useMemo(() => {
    return protocols.reduce<Record<string, Protocol>>((acc, protocol) => {
      acc[protocol.id] = protocol
      return acc
    }, {})
  }, [protocols])

  const normalizedSearch = search.trim().toLowerCase()

  const filteredConnections = useMemo(() => {
    return connections.filter((connection) => {
      // Filter by protocol if one is selected
      const matchesProtocol = !selectedProtocol || connection.protocol_id === selectedProtocol
      if (!matchesProtocol) {
        return false
      }

      // If no search, return true (show all matching protocol filter)
      if (!normalizedSearch) {
        return true
      }

      // Search filtering
      const protocol = protocolLookup[connection.protocol_id]
      const metadata = connection.metadata ?? {}
      const targets = connection.targets ?? []
      const rawTags = metadata.tags
      const tags = Array.isArray(rawTags)
        ? rawTags.filter((tag): tag is string => typeof tag === 'string')
        : []
      const hostMatches = targets.some((target) =>
        target.host.toLowerCase().includes(normalizedSearch)
      )
      const metadataMatch = Object.values(metadata).some(
        (value) => typeof value === 'string' && value.toLowerCase().includes(normalizedSearch)
      )
      const tagMatch = tags.some((tag: string) => tag.toLowerCase().includes(normalizedSearch))

      return (
        connection.name.toLowerCase().includes(normalizedSearch) ||
        (connection.description?.toLowerCase().includes(normalizedSearch) ?? false) ||
        hostMatches ||
        metadataMatch ||
        tagMatch ||
        protocol?.name.toLowerCase().includes(normalizedSearch)
      )
    })
  }, [connections, normalizedSearch, protocolLookup, selectedProtocol])

  const tabs: ProtocolTab[] = useMemo(() => {
    const counts = connections.reduce<Record<string, number>>((acc, connection) => {
      acc[connection.protocol_id] = (acc[connection.protocol_id] ?? 0) + 1
      return acc
    }, {})

    const protocolTabs: ProtocolTab[] = protocols
      .map((protocol) => ({
        id: protocol.id,
        label: protocol.name,
        icon: resolveProtocolIcon(protocol),
        count: counts[protocol.id] ?? 0,
        features: protocol.features,
      }))
      .filter((tab) => tab.count > 0) // Only show protocols that have connections

    return protocolTabs
  }, [connections, protocols])

  const isLoading = protocolsLoading || connectionsLoading
  const hasError = protocolsError || connectionsError

  return (
    <div className="flex h-full flex-col space-y-6 p-6">
      {/* Page Header */}
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-3xl font-bold tracking-tight">Connections</h1>
          <p className="text-sm text-muted-foreground">
            Manage and launch your infrastructure connections
          </p>
        </div>
        <Button asChild size="default" className="shadow-sm">
          <Link to="/connections/new">
            <Plus className="mr-2 h-4 w-4" />
            New Connection
          </Link>
        </Button>
      </header>

      {hasError && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 shadow-sm">
          <p className="text-sm font-medium text-destructive">
            Failed to load data. Check your network connection or permissions.
          </p>
        </div>
      )}

      {/* Search Bar */}
      <div className="rounded-lg border border-border/60 bg-card p-4 shadow-sm">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by name, host, tag, or protocol..."
            value={search}
            onChange={(event) => {
              const value = event.target.value
              setSearch(value)
              const params = new URLSearchParams(searchParams)
              if (value) {
                params.set('search', value)
              } else {
                params.delete('search')
              }
              setSearchParams(params, { replace: true })
            }}
            className="h-10 pl-9 pr-9"
          />
          {search && (
            <button
              onClick={() => {
                setSearch('')
                const params = new URLSearchParams(searchParams)
                params.delete('search')
                setSearchParams(params, { replace: true })
              }}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* Team Filter Tabs */}
      {canViewTeams && (
        <TeamFilterTabs
          teams={teams}
          connections={connections}
          activeTeam={teamParam}
          onTeamChange={(teamId) => {
            const params = new URLSearchParams(searchParams)
            if (teamId === 'all') {
              params.delete('team')
            } else {
              params.set('team', teamId)
            }
            params.delete('folder')
            setSearchParams(params, { replace: true })
          }}
        />
      )}

      {/* Main Content Area */}
      <div className="flex flex-1 gap-6 overflow-hidden">
        {/* Folders Sidebar */}
        <FolderSidebar
          folders={folderTree}
          activeFolderId={activeFolder}
          isLoading={foldersLoading}
          onFolderSelect={(folderId) => {
            const params = new URLSearchParams(searchParams)
            if (folderId) {
              params.set('folder', folderId)
            } else {
              params.delete('folder')
            }
            setSearchParams(params, { replace: true })
          }}
        />

        {/* Main Content */}
        <div className="flex min-w-0 flex-1 flex-col space-y-4 overflow-auto">
          {/* Protocol Tabs */}
          <ProtocolTabs
            tabs={tabs}
            isLoading={protocolsLoading}
            activeTab={selectedProtocol}
            onTabChange={setSelectedProtocol}
          />

          {/* Connections Grid or Empty/Loading State */}
          {isLoading ? (
            <LoadingState />
          ) : filteredConnections.length === 0 ? (
            <EmptyState hasProtocols={protocols.length > 0} search={normalizedSearch} />
          ) : (
            <div className="grid gap-4 pb-6 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
              {filteredConnections.map((connection) => (
                <ConnectionCard
                  key={connection.id}
                  connection={connection}
                  protocol={protocolLookup[connection.protocol_id]}
                  protocolIcon={resolveProtocolIcon(protocolLookup[connection.protocol_id])}
                  teamName={connection.team_id ? teamLookup[connection.team_id] : undefined}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

interface ProtocolTabsProps {
  tabs: ProtocolTab[]
  isLoading: boolean
  activeTab: string | null
  onTabChange: (tabId: string | null) => void
}

function ProtocolTabs({ tabs, isLoading, activeTab, onTabChange }: ProtocolTabsProps) {
  if (isLoading && !tabs.length) {
    return (
      <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-thin">
        {Array.from({ length: 4 }).map((_, index) => (
          <div
            key={`protocol-skeleton-${index}`}
            className="h-11 w-36 animate-pulse rounded-lg bg-muted"
          />
        ))}
      </div>
    )
  }

  if (tabs.length === 0) {
    return null // No protocol tabs to show
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Filter by Protocol
        </h3>
        {activeTab && (
          <button
            onClick={() => onTabChange(null)}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            Clear
          </button>
        )}
      </div>
      <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-thin">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={cn(
              'group flex shrink-0 items-center gap-2.5 whitespace-nowrap rounded-lg px-4 py-2.5 text-sm font-medium transition-all',
              activeTab === tab.id
                ? 'bg-primary text-primary-foreground shadow-md ring-2 ring-primary/20'
                : 'bg-card text-muted-foreground shadow-sm ring-1 ring-border/60 hover:bg-accent hover:text-accent-foreground hover:shadow'
            )}
          >
            <tab.icon className={cn('h-4 w-4 transition-transform group-hover:scale-110')} />
            <span>{tab.label}</span>
            <Badge
              variant={activeTab === tab.id ? 'secondary' : 'outline'}
              className={cn(
                'ml-0.5 text-xs font-semibold',
                activeTab === tab.id && 'bg-primary-foreground/20'
              )}
            >
              {tab.count}
            </Badge>
          </button>
        ))}
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
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 bg-muted/30 py-20 text-center">
      <div className="mb-4 rounded-full bg-muted p-4 ring-2 ring-border/40">
        <Server className="h-10 w-10 text-muted-foreground" />
      </div>
      <h3 className="mb-2 text-xl font-semibold">
        {search ? 'No matches found' : 'No connections yet'}
      </h3>
      <p className="mb-6 max-w-md text-sm text-muted-foreground">
        {search
          ? 'Try refining your search or switch to a different protocol tab.'
          : hasProtocols
            ? 'Create a connection to reuse driver settings and shared identities.'
            : 'No protocol drivers are currently available. Check your permissions or driver health.'}
      </p>
      {!search && hasProtocols && (
        <Button asChild size="default">
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
    <div className="grid gap-4 pb-6 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {Array.from({ length: 8 }).map((_, index) => (
        <div
          key={`connection-skeleton-${index}`}
          className="flex flex-col overflow-hidden rounded-lg border border-border/60 bg-card shadow-sm"
        >
          {/* Header skeleton */}
          <div className="border-b border-border/40 bg-muted/30 p-4">
            <div className="flex items-start gap-3">
              <div className="h-12 w-12 animate-pulse rounded-lg bg-muted" />
              <div className="flex-1 space-y-2">
                <div className="h-4 w-32 animate-pulse rounded bg-muted" />
                <div className="h-3 w-48 animate-pulse rounded bg-muted" />
              </div>
            </div>
          </div>
          {/* Body skeleton */}
          <div className="flex-1 space-y-3 p-4">
            <div className="flex gap-2">
              <div className="h-5 w-16 animate-pulse rounded bg-muted" />
              <div className="h-5 w-20 animate-pulse rounded bg-muted" />
            </div>
            <div className="h-3 w-full animate-pulse rounded bg-muted" />
            <div className="h-3 w-3/4 animate-pulse rounded bg-muted" />
          </div>
          {/* Footer skeleton */}
          <div className="border-t border-border/40 bg-muted/20 p-3">
            <div className="flex gap-2">
              <div className="h-9 flex-1 animate-pulse rounded bg-muted" />
              <div className="h-9 w-9 animate-pulse rounded bg-muted" />
            </div>
          </div>
        </div>
      ))}
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
