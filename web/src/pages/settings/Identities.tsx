import { Link } from 'react-router-dom'
import { Plus, Key, Search } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'

export function Identities() {
  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Identities & Credentials</h1>
          <p className="text-sm text-muted-foreground">
            Manage reusable credentials for your connections
          </p>
        </div>
        <Button asChild>
          <Link to="/settings/identities/new">
            <Plus className="h-4 w-4" />
            New Identity
          </Link>
        </Button>
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input placeholder="Search identities..." className="pl-9" />
      </div>

      {/* Empty State */}
      <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/50 py-16">
        <Key className="mb-4 h-12 w-12 text-muted-foreground" />
        <h3 className="mb-2 text-lg font-semibold">No identities found</h3>
        <p className="mb-4 text-sm text-muted-foreground">
          Create reusable credentials to use across your connections
        </p>
        <Button asChild>
          <Link to="/settings/identities/new">
            <Plus className="h-4 w-4" />
            Create Identity
          </Link>
        </Button>
      </div>
    </div>
  )
}
