import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, BarChart3, Clock, FolderTree, Layers, Plus, Zap } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { useAuth } from '@/hooks/useAuth'
import { useConnections } from '@/hooks/useConnections'
import { useConnectionFolders } from '@/hooks/useConnectionFolders'
import { useAvailableProtocols } from '@/hooks/useProtocols'
import type { ConnectionRecord } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

export function Dashboard() {
  const { user } = useAuth()
  const { data: connectionsResult, isLoading: connectionsLoading } = useConnections(
    { per_page: 100 },
    { staleTime: 30_000 }
  )
  const { data: folderTree = [] } = useConnectionFolders()
  const { data: protocols = [] } = useAvailableProtocols()

  const connections = connectionsResult?.data ?? []

  const totalConnections = connections.length
  const protocolCounts = useMemo(() => aggregateByProtocol(connections), [connections])
  const recentConnections = connections.slice(0, 6)

  const topProtocols = useMemo(() => {
    return Array.from(protocolCounts.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4)
  }, [protocolCounts])

  const totalFolders = useMemo(() => folderTree.length, [folderTree])

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Welcome back{user?.first_name ? `, ${user.first_name}` : ''}
          </h1>
          <p className="text-sm text-muted-foreground">
            Monitor your infrastructure access, organize folders, and jump into recent sessions.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" asChild size="sm">
            <Link to="/connections">
              View Connections
              <ArrowRight className="ml-1 h-4 w-4" />
            </Link>
          </Button>
          <Button asChild size="sm">
            <Link to="/connections/new">
              <Plus className="mr-1 h-4 w-4" />
              New Connection
            </Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Connections"
          value={totalConnections}
          icon={<Layers className="h-4 w-4 text-primary" />}
          description="Managed connection profiles"
          loading={connectionsLoading}
        />
        <StatCard
          title="Folders"
          value={totalFolders}
          icon={<FolderTree className="h-4 w-4 text-primary" />}
          description="Organized collections"
        />
        <StatCard
          title="Active Protocols"
          value={protocolCounts.size}
          icon={<Zap className="h-4 w-4 text-primary" />}
          description="Protocols with saved connections"
        />
        <StatCard
          title="Recent Updates"
          value={recentConnections.length}
          icon={<Clock className="h-4 w-4 text-primary" />}
          description="Modified this week"
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <div>
              <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Recent Connections
              </h2>
              <p className="text-xs text-muted-foreground">
                Jump back into a recently used resource.
              </p>
            </div>
            <Button variant="ghost" size="sm" asChild>
              <Link to="/connections">
                View all
                <ArrowRight className="ml-1 h-4 w-4" />
              </Link>
            </Button>
          </div>
          <div className="divide-y divide-border">
            {recentConnections.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-8 text-sm text-muted-foreground">
                No recent activity yet.
              </div>
            ) : (
              recentConnections.map((connection) => (
                <RecentConnection key={connection.id} connection={connection} />
              ))
            )}
          </div>
        </Card>

        <Card>
          <div className="border-b border-border px-4 py-3">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
              Protocol Mix
            </h2>
          </div>
          <div className="space-y-3 p-4">
            {topProtocols.length === 0 ? (
              <p className="text-sm text-muted-foreground">No protocols configured yet.</p>
            ) : (
              topProtocols.map(([protocolId, count]) => {
                const protocol = protocols.find((proto) => proto.id === protocolId)
                return (
                  <div key={protocolId} className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium">{protocol?.name ?? protocolId}</p>
                      <p className="text-xs text-muted-foreground">
                        {protocol?.category ?? 'Custom driver'}
                      </p>
                    </div>
                    <Badge variant="secondary">{count}</Badge>
                  </div>
                )
              })
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}

interface StatCardProps {
  title: string
  value: number
  description: string
  icon: React.ReactNode
  loading?: boolean
}

function StatCard({ title, value, description, icon, loading }: StatCardProps) {
  return (
    <Card className="flex flex-col gap-2 p-4">
      <div className="flex items-center justify-between text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        <span>{title}</span>
        {icon}
      </div>
      <div className="flex items-baseline gap-2">
        <p className="text-2xl font-bold">{loading ? '—' : value}</p>
      </div>
      <p className="text-xs text-muted-foreground">{description}</p>
    </Card>
  )
}

function RecentConnection({ connection }: { connection: ConnectionRecord }) {
  return (
    <div className="flex items-center justify-between px-4 py-3 hover:bg-muted/40">
      <div className="flex flex-col">
        <Link to={`/connections/${connection.id}`} className="font-medium hover:underline">
          {connection.name}
        </Link>
        <span className="text-xs uppercase tracking-wide text-muted-foreground">
          {connection.protocol_id}
        </span>
      </div>
      <Badge variant="outline" className="text-xs capitalize">
        {connection.folder?.name ?? 'Unassigned'}
      </Badge>
    </div>
  )
}

function aggregateByProtocol(connections: ConnectionRecord[]) {
  const map = new Map<string, number>()
  connections.forEach((connection) => {
    const count = map.get(connection.protocol_id) ?? 0
    map.set(connection.protocol_id, count + 1)
  })
  return map
}
