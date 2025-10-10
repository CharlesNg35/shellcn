import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Filter, Loader2, Search } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FolderTree } from '@/components/connections/FolderTree'
import { useConnectionFolders } from '@/hooks/useConnectionFolders'
import { useConnectionSummary } from '@/hooks/useConnectionSummary'
import { useConnections } from '@/hooks/useConnections'
import { useAvailableProtocols } from '@/hooks/useProtocols'
import type { ConnectionRecord } from '@/types/connections'

interface TeamConnectionsPanelProps {
  teamId: string
}

export function TeamConnectionsPanel({ teamId }: TeamConnectionsPanelProps) {
  const [activeFolder, setActiveFolder] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')

  const { data: folders = [], isLoading: foldersLoading } = useConnectionFolders(teamId)
  const { data: summary = [], isLoading: summaryLoading } = useConnectionSummary(teamId)
  const { data: protocolsResult } = useAvailableProtocols()
  const protocols = useMemo(() => protocolsResult?.data ?? [], [protocolsResult?.data])
  const protocolLookup = useMemo(() => {
    return protocols.reduce<Record<string, string>>((acc, protocol) => {
      acc[protocol.id] = protocol.name
      return acc
    }, {})
  }, [protocols])

  const { data: connectionsResult, isLoading: connectionsLoading } = useConnections(
    {
      team_id: teamId,
      folder_id: activeFolder ?? undefined,
      search: searchTerm.trim() || undefined,
      include: 'targets',
    },
    {
      enabled: Boolean(teamId),
    }
  )

  const connections = useMemo(() => connectionsResult?.data ?? [], [connectionsResult?.data])

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-sm lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="text-base font-semibold text-foreground">Team Connections</h2>
          <p className="text-sm text-muted-foreground">
            View and filter connections shared with this team. Folders and counts are scoped to the
            team membership.
          </p>
        </div>
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center">
          <div className="relative sm:w-64">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              placeholder="Search connections"
              className="pl-9"
            />
          </div>
          <Button variant="outline" className="gap-2" asChild>
            <Link to={`/connections?team_id=${encodeURIComponent(teamId)}`}>
              <Filter className="h-4 w-4" />
              Open in Connections
            </Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
        <div className="space-y-4">
          <div className="rounded-lg border border-border bg-card">
            <div className="flex items-center justify-between border-b border-border/80 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              <span>Folders</span>
              {foldersLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            </div>
            <div className="p-3">
              {foldersLoading ? (
                <div className="flex items-center gap-2 rounded-md bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading folders...
                </div>
              ) : (
                <FolderTree
                  nodes={folders}
                  activeFolderId={activeFolder}
                  onSelect={(folderId) => setActiveFolder(folderId)}
                  basePath={`/connections?team_id=${encodeURIComponent(teamId)}`}
                />
              )}
            </div>
          </div>

          <div className="rounded-lg border border-border bg-card">
            <div className="flex items-center justify-between border-b border-border/80 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              <span>Protocols</span>
              {summaryLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            </div>
            <div className="p-3 space-y-2">
              {summaryLoading ? (
                <div className="flex items-center gap-2 rounded-md bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading summary...
                </div>
              ) : summary.length === 0 ? (
                <p className="text-sm text-muted-foreground">No connections available yet.</p>
              ) : (
                summary
                  .filter((item) => item.count > 0)
                  .sort((a, b) => b.count - a.count)
                  .map((item) => (
                    <div
                      key={item.protocol_id}
                      className="flex items-center justify-between rounded-md border border-border/60 px-3 py-2 text-sm"
                    >
                      <span className="truncate">
                        {protocolLookup[item.protocol_id] ?? item.protocol_id.toUpperCase()}
                      </span>
                      <Badge variant="secondary" className="text-xs font-semibold">
                        {item.count}
                      </Badge>
                    </div>
                  ))
              )}
            </div>
          </div>
        </div>

        <div className="space-y-3">
          {connectionsLoading ? (
            <div className="flex items-center justify-center rounded-lg border border-border bg-card py-16 text-sm text-muted-foreground">
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              Loading team connections...
            </div>
          ) : connections.length === 0 ? (
            <div className="rounded-lg border border-border bg-card p-6 text-center">
              <h3 className="text-lg font-semibold text-foreground">No connections found</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                Try adjusting the search or switch folders to see more connections.
              </p>
            </div>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {connections.map((connection) => (
                <ConnectionTile key={connection.id} connection={connection} protocolLookup={protocolLookup} />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

interface ConnectionTileProps {
  connection: ConnectionRecord
  protocolLookup: Record<string, string>
}

function ConnectionTile({ connection, protocolLookup }: ConnectionTileProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm transition hover:shadow">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold text-foreground">{connection.name}</h3>
          {connection.description ? (
            <p className="text-sm text-muted-foreground line-clamp-2">{connection.description}</p>
          ) : null}
        </div>
        <Badge variant="outline" className="text-[11px] uppercase tracking-wide">
          {protocolLookup[connection.protocol_id] ?? connection.protocol_id.toUpperCase()}
        </Badge>
      </div>

      <div className="mt-3 space-y-1 text-xs text-muted-foreground">
        {connection.targets?.length ? (
          <p className="truncate">
            Primary target: {connection.targets[0].host}
            {connection.targets[0].port ? `:${connection.targets[0].port}` : ''}
          </p>
        ) : null}
        {connection.last_used_at ? (
          <p>Last used {new Date(connection.last_used_at).toLocaleString()}</p>
        ) : (
          <p>Never used</p>) }
      </div>
    </div>
  )
}
