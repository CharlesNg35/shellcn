import { Building2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function Organizations() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Organizations</h1>
          <p className="mt-2 text-muted-foreground">Manage organizational units and hierarchies</p>
        </div>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          New Organization
        </Button>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Building2 className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">Organization Management</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Organization management functionality will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
