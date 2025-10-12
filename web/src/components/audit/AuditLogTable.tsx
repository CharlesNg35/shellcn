import { useMemo, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  useReactTable,
} from '@tanstack/react-table'
import { format } from 'date-fns'
import { ArrowUpDown, Eye, Loader2 } from 'lucide-react'
import type { AuditLogEntry } from '@/types/audit'
import type { ApiMeta } from '@/types/api'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { Skeleton } from '@/components/ui/Skeleton'
import { cn } from '@/lib/utils/cn'

interface AuditLogTableProps {
  logs: AuditLogEntry[]
  meta?: ApiMeta
  page: number
  perPage: number
  isLoading?: boolean
  isFetching?: boolean
  onPageChange: (page: number) => void
  onSelectLog?: (log: AuditLogEntry) => void
}

function formatTimestamp(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  try {
    return format(date, 'PPpp')
  } catch {
    return date.toLocaleString()
  }
}

function getResultVariant(result: string | undefined) {
  const normalized = (result ?? '').toLowerCase()
  switch (normalized) {
    case 'success':
      return 'success'
    case 'failure':
    case 'denied':
    case 'error':
      return 'destructive'
    default:
      return 'secondary'
  }
}

function getResultLabel(result: string | undefined) {
  if (!result) {
    return 'Unknown'
  }
  return result.charAt(0).toUpperCase() + result.slice(1)
}

export function AuditLogTable({
  logs,
  meta,
  page,
  perPage,
  isLoading,
  isFetching,
  onPageChange,
  onSelectLog,
}: AuditLogTableProps) {
  const [sorting, setSorting] = useState<SortingState>([{ id: 'created_at', desc: true }])

  const columns = useMemo<ColumnDef<AuditLogEntry>[]>(() => {
    return [
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="-ml-3"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Timestamp
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => (
          <div className="flex flex-col text-xs">
            <span className="font-medium text-foreground">
              {formatTimestamp(row.original.created_at)}
            </span>
            <span className="text-muted-foreground">{row.original.id}</span>
          </div>
        ),
        size: 220,
      },
      {
        accessorKey: 'username',
        header: () => 'Actor',
        cell: ({ row }) => {
          const { username, user } = row.original
          return (
            <div className="flex flex-col text-sm">
              <span className="font-medium text-foreground">{username}</span>
              {user?.email ? (
                <span className="text-xs text-muted-foreground">{user.email}</span>
              ) : null}
            </div>
          )
        },
      },
      {
        accessorKey: 'action',
        header: () => 'Action',
        cell: ({ row }) => (
          <div className="flex flex-col text-sm">
            <span className="font-medium text-foreground">{row.original.action}</span>
            {row.original.resource ? (
              <span className="text-xs text-muted-foreground">{row.original.resource}</span>
            ) : null}
          </div>
        ),
      },
      {
        accessorKey: 'result',
        header: () => 'Result',
        cell: ({ row }) => (
          <Badge variant={getResultVariant(row.original.result)}>
            {getResultLabel(row.original.result)}
          </Badge>
        ),
        size: 110,
      },
      {
        accessorKey: 'ip_address',
        header: () => 'IP Address',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">{row.original.ip_address ?? '—'}</span>
        ),
        size: 140,
      },
      {
        accessorKey: 'user_agent',
        header: () => 'User Agent',
        cell: ({ row }) => (
          <span className="line-clamp-2 text-xs text-muted-foreground">
            {row.original.user_agent ?? '—'}
          </span>
        ),
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">View</span>,
        cell: ({ row }) => (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-2 hover:bg-muted"
            onClick={() => onSelectLog?.(row.original)}
          >
            <Eye className="h-4 w-4" />
            Details
          </Button>
        ),
        enableSorting: false,
        size: 120,
      },
    ]
  }, [onSelectLog])

  const table = useReactTable({
    data: logs,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  const total = meta?.total ?? logs.length
  const totalPages = meta?.total_pages ?? (total > 0 ? Math.ceil(total / perPage) : 1)

  const canPreviousPage = page > 1
  const canNextPage = meta?.total_pages ? page < meta.total_pages : logs.length === perPage

  const showingFrom = total === 0 ? 0 : (page - 1) * perPage + 1
  const showingTo = total === 0 ? 0 : Math.min(page * perPage, total)

  return (
    <div className="overflow-hidden rounded-lg border border-border/70 bg-card shadow-sm">
      <div className="relative overflow-x-auto">
        <table className="w-full border-collapse text-left">
          <thead className="bg-muted/60 text-xs uppercase tracking-wide text-muted-foreground">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id} className="px-4 py-3 font-medium">
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody className="divide-y divide-border/70 text-sm">
            {isLoading && logs.length === 0 ? (
              Array.from({ length: 5 }).map((_, index) => (
                <tr key={`skeleton-${index}`}>
                  <td className="px-4 py-4" colSpan={columns.length}>
                    <Skeleton className="h-5 w-full" />
                  </td>
                </tr>
              ))
            ) : table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={cn('transition-colors hover:bg-muted/40', isFetching && 'opacity-80')}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-4 py-4 align-top">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-4 py-8" colSpan={columns.length}>
                  <EmptyState
                    title="No audit events found"
                    description="Adjust your filters or try a different search query to see audit activity."
                    className="min-h-[200px] border-dashed"
                  />
                </td>
              </tr>
            )}
          </tbody>
        </table>

        {isFetching && logs.length > 0 ? (
          <div className="pointer-events-none absolute inset-x-0 top-0 flex justify-center">
            <div className="flex items-center gap-2 rounded-b-lg bg-background/90 px-4 py-2 text-xs text-muted-foreground shadow">
              <Loader2 className="h-4 w-4 animate-spin" />
              Updating results…
            </div>
          </div>
        ) : null}
      </div>

      <div className="flex flex-col gap-3 border-t border-border/60 px-4 py-4 text-sm text-muted-foreground md:flex-row md:items-center md:justify-between">
        <div>
          {total === 0 ? (
            <span>No results</span>
          ) : (
            <span>
              Showing {showingFrom.toLocaleString()} - {showingTo.toLocaleString()} of{' '}
              {total.toLocaleString()}
            </span>
          )}
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onPageChange(page - 1)}
              disabled={!canPreviousPage}
            >
              Previous
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onPageChange(page + 1)}
              disabled={!canNextPage}
            >
              Next
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
