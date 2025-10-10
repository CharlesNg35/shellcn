import { useEffect, useMemo, useState } from 'react'
import { Filter, RefreshCcw } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import type { UserListParams } from '@/types/users'

export interface UserFilterState {
  search?: string
  status?: 'all' | 'active' | 'inactive'
}

interface UserFiltersProps {
  filters: UserFilterState
  onChange: (filters: UserFilterState) => void
}

const STATUS_FILTERS: Array<{ label: string; value: UserFilterState['status'] }> = [
  { label: 'All', value: 'all' },
  { label: 'Active', value: 'active' },
  { label: 'Inactive', value: 'inactive' },
]

// eslint-disable-next-line react-refresh/only-export-components
export function normalizeFilters(filters: UserFilterState): UserListParams {
  const normalized: UserListParams = {}
  if (filters.search) {
    normalized.search = filters.search
  }
  if (filters.status) {
    normalized.status = filters.status
  }
  return normalized
}

export function UserFilters({ filters, onChange }: UserFiltersProps) {
  const [searchValue, setSearchValue] = useState(filters.search ?? '')

  useEffect(() => {
    setSearchValue(filters.search ?? '')
  }, [filters.search])

  useEffect(() => {
    const handle = window.setTimeout(() => {
      if (searchValue === filters.search) {
        return
      }
      onChange({ ...filters, search: searchValue.trim() || undefined })
    }, 250)

    return () => {
      window.clearTimeout(handle)
    }
  }, [filters, onChange, searchValue])

  const activeStatus = filters.status ?? 'all'

  const resetDisabled = useMemo(() => {
    return !filters.search && (!filters.status || filters.status === 'all')
  }, [filters.search, filters.status])

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border/60 bg-card p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          <Filter className="h-4 w-4" />
          Filters
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={resetDisabled}
          onClick={() => onChange({ search: undefined, status: 'all' })}
        >
          <RefreshCcw className="mr-2 h-4 w-4" />
          Reset
        </Button>
      </div>

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <Input
          label="Search"
          placeholder="Search by username, email, or name"
          value={searchValue}
          onChange={(event) => setSearchValue(event.target.value)}
        />

        <div className="flex flex-1 flex-wrap gap-2">
          {STATUS_FILTERS.map((status) => {
            const isActive = activeStatus === status.value
            return (
              <Button
                key={status.value}
                type="button"
                variant={isActive ? 'default' : 'outline'}
                size="sm"
                onClick={() => onChange({ ...filters, status: status.value })}
              >
                {status.label}
              </Button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
