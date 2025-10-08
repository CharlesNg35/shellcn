import { Link } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import {
  Server,
  Monitor,
  Database,
  Container,
  Cloud,
  HardDrive,
  Activity,
  Plus,
  ArrowRight,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function Dashboard() {
  const { user } = useAuth()

  const connectionTypes = [
    {
      name: 'SSH / Telnet',
      count: 0,
      icon: Server,
      color: 'text-blue-500',
      bgColor: 'bg-blue-500/10',
      to: '/ssh',
    },
    {
      name: 'RDP',
      count: 0,
      icon: Monitor,
      color: 'text-purple-500',
      bgColor: 'bg-purple-500/10',
      to: '/rdp',
    },
    {
      name: 'VNC',
      count: 0,
      icon: Monitor,
      color: 'text-green-500',
      bgColor: 'bg-green-500/10',
      to: '/vnc',
    },
    {
      name: 'Docker',
      count: 0,
      icon: Container,
      color: 'text-cyan-500',
      bgColor: 'bg-cyan-500/10',
      to: '/docker',
    },
    {
      name: 'Kubernetes',
      count: 0,
      icon: Cloud,
      color: 'text-indigo-500',
      bgColor: 'bg-indigo-500/10',
      to: '/kubernetes',
    },
    {
      name: 'Databases',
      count: 0,
      icon: Database,
      color: 'text-orange-500',
      bgColor: 'bg-orange-500/10',
      to: '/databases',
    },
    {
      name: 'Proxmox',
      count: 0,
      icon: HardDrive,
      color: 'text-red-500',
      bgColor: 'bg-red-500/10',
      to: '/proxmox',
    },
  ]

  const recentActivity: unknown[] = [
    // Mock data - will be replaced with API
  ]

  return (
    <div className="space-y-6">
      {/* Welcome header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Welcome back{user?.first_name ? `, ${user.first_name}` : ''}
          </h1>
          <p className="text-sm text-muted-foreground">
            Manage your remote connections and infrastructure access
          </p>
        </div>
        <Button asChild>
          <Link to="/connections/new">
            <Plus className="h-4 w-4" />
            New Connection
          </Link>
        </Button>
      </div>

      {/* Connection Types Grid */}
      <div>
        <h2 className="mb-4 text-lg font-semibold">Connection Types</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {connectionTypes.map((type) => (
            <Link
              key={type.name}
              to={type.to}
              className="group rounded-lg border border-border bg-card p-6 shadow-sm transition-all hover:shadow-md"
            >
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <p className="text-sm font-medium text-muted-foreground">{type.name}</p>
                  <p className="text-2xl font-bold tracking-tight">{type.count}</p>
                </div>
                <div
                  className={`flex h-10 w-10 items-center justify-center rounded-md ${type.bgColor}`}
                >
                  <type.icon className={`h-5 w-5 ${type.color}`} />
                </div>
              </div>
              <div className="mt-4 flex items-center text-sm text-muted-foreground group-hover:text-foreground">
                View all
                <ArrowRight className="ml-1 h-4 w-4" />
              </div>
            </Link>
          ))}
        </div>
      </div>

      {/* Recent Activity */}
      <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Recent Activity</h2>
            <p className="text-sm text-muted-foreground">Latest connections and events</p>
          </div>
          <Button variant="outline" size="sm" asChild>
            <Link to="/settings/audit">
              View All
              <ArrowRight className="ml-2 h-4 w-4" />
            </Link>
          </Button>
        </div>
        <div className="mt-6">
          {recentActivity.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Activity className="mb-3 h-12 w-12 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">No recent activity</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Your connection history will appear here
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {recentActivity.map((_activity, index) => (
                <div
                  key={index}
                  className="flex items-center gap-3 rounded-md border border-border p-3"
                >
                  {/* Activity items will go here */}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Getting started */}
      <div className="rounded-lg border border-primary/20 bg-primary/5 p-6">
        <h3 className="font-semibold">Getting Started</h3>
        <p className="mt-2 text-sm text-muted-foreground">
          Create your first connection to get started with remote access. You can connect to SSH
          servers, RDP desktops, Docker hosts, Kubernetes clusters, databases, and more.
        </p>
        <div className="mt-4 flex gap-2">
          <Button size="sm" asChild>
            <Link to="/connections/new">Create Connection</Link>
          </Button>
          <Button size="sm" variant="outline" asChild>
            <Link to="/settings/identities">Manage Credentials</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
