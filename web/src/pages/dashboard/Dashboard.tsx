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
    <div className="space-y-6">
      {/* Welcome header */}
      <div>
        <h1 className="text-3xl font-bold text-foreground">
          Welcome back{user?.first_name ? `, ${user.first_name}` : ''}!
        </h1>
        <p className="mt-2 text-muted-foreground">
          Here's an overview of your infrastructure access platform
        </p>
      </div>

      {/* Stats grid */}
      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <div key={stat.name} className="rounded-lg border border-border bg-card p-6 shadow-sm">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">{stat.name}</p>
                <p className="mt-2 text-3xl font-bold text-foreground">{stat.value}</p>
              </div>
              <div
                className={`flex h-12 w-12 items-center justify-center rounded-lg ${stat.bgColor}`}
              >
                <stat.icon className={`h-6 w-6 ${stat.color}`} />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Quick actions */}
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-foreground">Quick Actions</h2>
          <p className="mt-1 text-sm text-muted-foreground">Common tasks and shortcuts</p>
          <div className="mt-4 space-y-2">
            <div className="rounded-md border border-border p-3 text-sm text-muted-foreground">
              • Add new connection
            </div>
            <div className="rounded-md border border-border p-3 text-sm text-muted-foreground">
              • Invite team members
            </div>
            <div className="rounded-md border border-border p-3 text-sm text-muted-foreground">
              • Configure authentication
            </div>
          </div>
        </div>

        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-foreground">Recent Activity</h2>
          <p className="mt-1 text-sm text-muted-foreground">Latest events and actions</p>
          <div className="mt-4 flex items-center justify-center py-8 text-sm text-muted-foreground">
            No recent activity
          </div>
        </div>
      </div>

      {/* Getting started */}
      <div className="rounded-lg border border-primary/20 bg-primary/5 p-6">
        <h2 className="text-lg font-semibold text-foreground">Getting Started</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          This is a placeholder dashboard. Full functionality including connection management,
          terminal access, and monitoring will be available in Phase 3.
        </p>
      </div>
    </div>
  )
}
