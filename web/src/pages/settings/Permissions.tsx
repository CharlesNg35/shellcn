import { Shield } from 'lucide-react'

export function Permissions() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">Permissions</h1>
        <p className="mt-2 text-muted-foreground">Manage roles and permissions</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Shield className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">Permission Management</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Permission management functionality will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
