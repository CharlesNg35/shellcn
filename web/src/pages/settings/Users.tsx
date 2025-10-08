import { Users as UsersIcon, UserPlus } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function Users() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Users</h1>
          <p className="mt-2 text-muted-foreground">Manage user accounts and permissions</p>
        </div>
        <Button>
          <UserPlus className="mr-2 h-4 w-4" />
          Add User
        </Button>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <UsersIcon className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">User Management</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          User management functionality will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
