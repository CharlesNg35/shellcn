import { Link } from 'react-router-dom'
import { Activity, ArrowRightLeft, Bell } from 'lucide-react'
import { useAuth } from '@/hooks/useAuth'
import { useConnections } from '@/hooks/useConnections'
import { useNotifications } from '@/hooks/useNotifications'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'

export function Dashboard() {
  const { user } = useAuth()
  const { data: connections } = useConnections({ per_page: 1 })
  const { unreadCount } = useNotifications()

  const connectionsTotal = connections?.meta?.total ?? connections?.data?.length ?? 0

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          Welcome back{user ? `, ${user.first_name ?? user.username}` : ''}
        </h1>
        <p className="text-muted-foreground">Here’s a quick overview of your ShellCN workspace.</p>
      </div>

      <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connections</CardTitle>
            <ArrowRightLeft className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{connectionsTotal}</div>
            <p className="text-xs text-muted-foreground">
              Visible to you based on permissions.{' '}
              <Link to="/connections" className="text-primary hover:underline">
                View all
              </Link>
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Unread notifications</CardTitle>
            <Bell className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{unreadCount}</div>
            <p className="text-xs text-muted-foreground">Real-time updates via WebSocket</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Activity</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <CardDescription>Recent activity widgets coming soon.</CardDescription>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
