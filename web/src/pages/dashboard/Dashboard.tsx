import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, Clock, FolderTree, Layers, Plus, Zap } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { PageHeader } from '@/components/layout/PageHeader'
import { useAuth } from '@/hooks/useAuth'
import { useConnections } from '@/hooks/useConnections'
import { useConnectionFolders } from '@/hooks/useConnectionFolders'
import { useAvailableProtocols } from '@/hooks/useProtocols'
import type { ConnectionRecord } from '@/types/connections'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PERMISSIONS } from '@/constants/permissions'

export function Dashboard() {
  const { user } = useAuth()
  const { data: connectionsResult, isLoading: connectionsLoading } = useConnections(
    { per_page: 100 },
    { staleTime: 30_000 }
  )
  const { data: folderTree = [] } = useConnectionFolders()
  const { data: availableProtocols } = useAvailableProtocols()
  const protocols = availableProtocols?.data ?? []

  const connections = useMemo(() => connectionsResult?.data ?? [], [connectionsResult?.data])

  const totalConnections = connections.length
  const protocolCounts = useMemo(() => aggregateByProtocol(connections), [connections])
  const recentConnections = useMemo(() => connections.slice(0, 6), [connections])

  const topProtocols = useMemo(() => {
    return Array.from(protocolCounts.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4)
  }, [protocolCounts])

  const totalFolders = useMemo(() => folderTree.length, [folderTree])

  const greeting = user?.first_name ? `Welcome back, ${user.first_name}` : 'Welcome back'

  return (
    <div className="space-y-6">
      <PageHeader
        title={greeting}
        description="Monitor your infrastructure access, organize folders, and jump into recent sessions. Your central hub for managing all remote connections."
        action={
          <div className="flex flex-wrap gap-2">
            <PermissionGuard permission={PERMISSIONS.CONNECTION.VIEW}>
              <Button variant="outline" asChild size="sm">
                <Link to="/connections">
                  View All Connections
                  <ArrowRight className="ml-2 h-4 w-4" />
                </Link>
              </Button>
            </PermissionGuard>
            <PermissionGuard permission={PERMISSIONS.CONNECTION.MANAGE}>
              <Button asChild size="sm">
                <Link to="/connections/new">
                  <Plus className="mr-2 h-4 w-4" />
                  New Connection
                </Link>
              </Button>
            </PermissionGuard>
          </div>
        }
      />

      {/* Statistics Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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

      {/* Main Content Grid */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Recent Connections */}
        <Card className="lg:col-span-2">
          <CardHeader className="border-b border-border">
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <CardTitle className="text-base">Recent Connections</CardTitle>
                <CardDescription>Jump back into a recently used resource</CardDescription>
              </div>
              <PermissionGuard permission={PERMISSIONS.CONNECTION.VIEW}>
                <Button variant="ghost" size="sm" asChild>
                  <Link to="/connections">
                    View all
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </Link>
                </Button>
              </PermissionGuard>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            {recentConnections.length === 0 ? (
              <div className="flex min-h-[200px] flex-col items-center justify-center p-8 text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                  <Layers className="h-6 w-6 text-muted-foreground" />
                </div>
                <p className="mt-4 text-sm font-medium text-foreground">No connections yet</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  Create your first connection to get started
                </p>
                <PermissionGuard permission={PERMISSIONS.CONNECTION.MANAGE}>
                  <Button asChild size="sm" className="mt-4">
                    <Link to="/connections/new">
                      <Plus className="mr-2 h-4 w-4" />
                      Create Connection
                    </Link>
                  </Button>
                </PermissionGuard>
              </div>
            ) : (
              <div className="divide-y divide-border">
                {recentConnections.map((connection) => (
                  <RecentConnection key={connection.id} connection={connection} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Protocol Mix */}
        <Card>
          <CardHeader className="border-b border-border">
            <CardTitle className="text-base">Protocol Mix</CardTitle>
            <CardDescription>Your most used protocols</CardDescription>
          </CardHeader>
          <CardContent className="p-4">
            {topProtocols.length === 0 ? (
              <div className="flex min-h-[200px] flex-col items-center justify-center text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                  <Zap className="h-6 w-6 text-muted-foreground" />
                </div>
                <p className="mt-4 text-sm text-muted-foreground">No protocols configured yet</p>
              </div>
            ) : (
              <div className="space-y-4">
                {topProtocols.map(([protocolId, count]) => {
                  const protocol = protocols.find((proto) => proto.id === protocolId)
                  return (
                    <div key={protocolId} className="flex items-center justify-between gap-4">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-foreground">
                          {protocol?.name ?? protocolId}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {protocol?.category ?? 'Custom driver'}
                        </p>
                      </div>
                      <Badge variant="secondary" className="shrink-0">
                        {count}
                      </Badge>
                    </div>
                  )
                })}
              </div>
            )}
          </CardContent>
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
    <Card>
      <CardContent className="p-6">
        <div className="flex items-center justify-between">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {title}
          </p>
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
            {icon}
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <p className="text-2xl font-bold text-foreground">{loading ? '—' : value}</p>
        </div>
        <p className="mt-1 text-xs text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  )
}

function RecentConnection({ connection }: { connection: ConnectionRecord }) {
  return (
    <Link
      to={`/connections/${connection.id}`}
      className="flex items-center justify-between px-6 py-4 transition hover:bg-muted/40"
    >
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-foreground hover:underline">{connection.name}</p>
        <p className="mt-0.5 truncate text-xs uppercase tracking-wide text-muted-foreground">
          {connection.protocol_id}
        </p>
      </div>
      <Badge variant="outline" className="ml-4 shrink-0 text-xs capitalize">
        {connection.folder?.name ?? 'Unassigned'}
      </Badge>
    </Link>
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
