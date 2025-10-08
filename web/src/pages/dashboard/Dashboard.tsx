import { useAuth } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'

export function Dashboard() {
  const { user, logout } = useAuth()

  return (
    <div className="p-6">
      <div className="rounded-2xl border border-border bg-card p-8 shadow-lg shadow-black/5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-foreground">
              Welcome back{user?.first_name ? `, ${user.first_name}` : ''}!
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              This is a placeholder dashboard. Navigation and widgets will arrive in Phase&nbsp;3.
            </p>
          </div>
          <Button
            variant="outline"
            onClick={() => {
              void logout()
            }}
          >
            Sign out
          </Button>
        </div>
      </div>
    </div>
  )
}
