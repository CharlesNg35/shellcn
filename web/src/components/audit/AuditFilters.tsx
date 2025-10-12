import { useEffect, useMemo, useState } from 'react'
import { Filter, RefreshCcw } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import type { AuditLogResult } from '@/types/audit'

export interface AuditFilterState {
  search?: string
  action?: string
  resource?: string
  result?: AuditLogResult | 'all'
  actor?: string
  from?: string
  to?: string
}

interface AuditFiltersProps {
  filters: AuditFilterState
  onChange: (filters: AuditFilterState) => void
}

const RESULT_OPTIONS: Array<{ label: string; value: AuditFilterState['result'] }> = [
  { label: 'All results', value: 'all' },
  { label: 'Success', value: 'success' },
  { label: 'Failure', value: 'failure' },
  { label: 'Denied', value: 'denied' },
  { label: 'Error', value: 'error' },
]

function normalizeValue(value?: string) {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}

export function AuditFilters({ filters, onChange }: AuditFiltersProps) {
  const [searchValue, setSearchValue] = useState(filters.search ?? '')

  useEffect(() => {
    setSearchValue(filters.search ?? '')
  }, [filters.search])

  useEffect(() => {
    const handle = window.setTimeout(() => {
      if (searchValue === filters.search) {
        return
      }
      onChange({ ...filters, search: normalizeValue(searchValue) })
    }, 250)

    return () => {
      window.clearTimeout(handle)
    }
  }, [filters, onChange, searchValue])

  const resetDisabled = useMemo(() => {
    const { search, action, resource, result, actor, from, to } = filters
    return (
      !search && !action && !resource && (!result || result === 'all') && !actor && !from && !to
    )
  }, [filters])

  const activeResult = filters.result ?? 'all'

  return (
    <div className="space-y-5 rounded-lg border border-border/60 bg-card p-5 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Filter className="h-4 w-4" />
          <span>Filter audit logs</span>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={resetDisabled}
          onClick={() =>
            onChange({
              result: 'all',
            })
          }
        >
          <RefreshCcw className="mr-2 h-4 w-4" />
          Reset
        </Button>
      </div>

      <div className="space-y-4">
        <Input
          label="Search"
          aria-label="Search audit logs"
          placeholder="Search by actor, action, resource, or metadata…"
          value={searchValue}
          onChange={(event) => setSearchValue(event.target.value)}
        />

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Input
            label="Action"
            placeholder="e.g. user.create"
            value={filters.action ?? ''}
            onChange={(event) =>
              onChange({ ...filters, action: normalizeValue(event.target.value) })
            }
          />
          <Input
            label="Resource"
            placeholder="e.g. user:usr_123"
            value={filters.resource ?? ''}
            onChange={(event) =>
              onChange({ ...filters, resource: normalizeValue(event.target.value) })
            }
          />
          <Input
            label="Actor"
            placeholder="Username or email"
            value={filters.actor ?? ''}
            onChange={(event) =>
              onChange({ ...filters, actor: normalizeValue(event.target.value) })
            }
          />
          <div className="flex flex-col gap-2">
            <label
              className="text-xs font-medium text-muted-foreground"
              htmlFor="audit-result-filter"
            >
              Result
            </label>
            <select
              id="audit-result-filter"
              className="h-10 rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              value={activeResult}
              onChange={(event) =>
                onChange({
                  ...filters,
                  result: event.target.value as AuditFilterState['result'],
                })
              }
            >
              {RESULT_OPTIONS.map((option) => (
                <option key={option.value ?? 'all'} value={option.value ?? 'all'}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <Input
            label="From date"
            type="date"
            value={filters.from ?? ''}
            onChange={(event) => onChange({ ...filters, from: event.target.value || undefined })}
            aria-label="Filter from date"
          />
          <Input
            label="To date"
            type="date"
            value={filters.to ?? ''}
            onChange={(event) => onChange({ ...filters, to: event.target.value || undefined })}
            aria-label="Filter to date"
          />
        </div>
      </div>
    </div>
  )
}
