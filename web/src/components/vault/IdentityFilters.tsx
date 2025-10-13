import { Search as SearchIcon, SlidersHorizontal } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import type { IdentityScope } from '@/types/vault'
import { cn } from '@/lib/utils/cn'

type IdentityScopeFilter = IdentityScope | 'all'

interface ProtocolOption {
  id: string
  name: string
}

interface IdentityFiltersProps {
  search: string
  scope: IdentityScopeFilter
  protocolId: string | 'all'
  includeConnectionScoped: boolean
  onSearchChange: (value: string) => void
  onScopeChange: (scope: IdentityScopeFilter) => void
  onProtocolChange: (protocolId: string | 'all') => void
  onIncludeConnectionScopedChange: (value: boolean) => void
  protocolOptions: ProtocolOption[]
}

const SCOPE_OPTIONS: Array<{ label: string; value: IdentityScopeFilter }> = [
  { label: 'All', value: 'all' },
  { label: 'Global', value: 'global' },
  { label: 'Team', value: 'team' },
  { label: 'Connection', value: 'connection' },
]

export function IdentityFilters({
  search,
  scope,
  protocolId,
  includeConnectionScoped,
  onSearchChange,
  onScopeChange,
  onProtocolChange,
  onIncludeConnectionScopedChange,
  protocolOptions,
}: IdentityFiltersProps) {
  return (
    <div className="bg-card border border-border rounded-lg p-4 space-y-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-1 items-center gap-2">
          <div className="relative flex-1">
            <SearchIcon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
              placeholder="Search identities by name, description, or owner..."
              className="pl-9"
            />
          </div>
          <div className="hidden lg:flex lg:items-center lg:gap-2 text-sm text-muted-foreground">
            <SlidersHorizontal className="h-4 w-4" />
            Filters
          </div>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 lg:flex lg:flex-none lg:items-center lg:gap-3">
          <div className="flex items-center gap-3">
            <label htmlFor="identity-protocol-filter" className="text-xs font-medium uppercase">
              Protocol
            </label>
            <select
              id="identity-protocol-filter"
              className="h-9 rounded-md border border-border bg-background px-3 text-sm"
              value={protocolId}
              onChange={(event) => onProtocolChange(event.target.value as string | 'all')}
            >
              <option value="all">All protocols</option>
              {protocolOptions.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.name}
                </option>
              ))}
            </select>
          </div>
          <label className="flex items-center gap-2 text-sm text-foreground">
            <Checkbox
              checked={includeConnectionScoped}
              onCheckedChange={(checked) => onIncludeConnectionScopedChange(Boolean(checked))}
            />
            Include connection scoped
          </label>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {SCOPE_OPTIONS.map((option) => (
          <Button
            key={option.value}
            type="button"
            variant={scope === option.value ? 'default' : 'outline'}
            size="sm"
            className={cn('rounded-full px-4 text-sm', scope === option.value ? 'shadow-sm' : '')}
            onClick={() => onScopeChange(option.value)}
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  )
}

export type { IdentityScopeFilter }
