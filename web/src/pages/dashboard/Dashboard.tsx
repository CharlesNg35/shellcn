import { useAuth } from '@/hooks/useAuth'
import { Server, Users, Activity, Shield } from 'lucide-react'

export function Dashboard() {
  const { user } = useAuth()

  const stats = [
    {
      name: 'Active Connections',
      value: '0',
      icon: Server,
      color: 'text-primary',
      bgColor: 'bg-primary/10',
    },
    {
      name: 'Total Users',
      value: '1',
      icon: Users,
      color: 'text-accent',
      bgColor: 'bg-accent/10',
    },
    {
      name: 'Active Sessions',
      value: '0',
      icon: Activity,
      color: 'text-chart-3',
      bgColor: 'bg-chart-3/10',
    },
    {
      name: 'Security Events',
      value: '0',
      icon: Shield,
      color: 'text-secondary',
      bgColor: 'bg-secondary/10',
    },
  ]

  return (
    <div className="space-y-4">
      {/* Welcome header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          Welcome back{user?.first_name ? `, ${user.first_name}` : ''}
        </h1>
        <p className="text-sm text-muted-foreground">
          Here's an overview of your infrastructure access platform
        </p>
      </div>

      {/* Stats grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <div key={stat.name} className="rounded-lg border border-border bg-card p-6 shadow-sm">
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <p className="text-sm font-medium text-muted-foreground">{stat.name}</p>
                <p className="text-2xl font-bold tracking-tight">{stat.value}</p>
              </div>
              <div
                className={`flex h-10 w-10 items-center justify-center rounded-md ${stat.bgColor}`}
              >
                <stat.icon className={`h-5 w-5 ${stat.color}`} />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Quick actions */}
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <h3 className="font-semibold">Quick Actions</h3>
          <p className="mt-1 text-sm text-muted-foreground">Common tasks and shortcuts</p>
          <div className="mt-4 space-y-2">
            <button className="flex w-full items-center gap-2 rounded-md border border-border px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground">
              <span className="text-muted-foreground">•</span>
              Add new connection
            </button>
            <button className="flex w-full items-center gap-2 rounded-md border border-border px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground">
              <span className="text-muted-foreground">•</span>
              Invite team members
            </button>
            <button className="flex w-full items-center gap-2 rounded-md border border-border px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground">
              <span className="text-muted-foreground">•</span>
              Configure authentication
            </button>
          </div>
        </div>

        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <h3 className="font-semibold">Recent Activity</h3>
          <p className="mt-1 text-sm text-muted-foreground">Latest events and actions</p>
          <div className="mt-4 flex items-center justify-center py-8 text-sm text-muted-foreground">
            No recent activity
          </div>
        </div>
      </div>

      {/* Getting started */}
      <div className="rounded-lg border border-primary/20 bg-primary/5 p-6">
        <h3 className="font-semibold">Getting Started</h3>
        <p className="mt-2 text-sm text-muted-foreground">
          This is a placeholder dashboard. Full functionality including connection management,
          terminal access, and monitoring will be available in Phase 3.
        </p>
      </div>
    </div>
  )
}
