import { useMemo, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  useReactTable,
} from '@tanstack/react-table'
import { ArrowUpDown, Eye, Layers, PencilLine, Share2, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { IdentityScopeBadge } from '@/components/vault/IdentityScopeBadge'
import type { IdentityRecord } from '@/types/vault'
import { cn } from '@/lib/utils/cn'

interface IdentityTableProps {
  identities: IdentityRecord[]
  isLoading?: boolean
  templateNames?: Record<string, string>
  onViewIdentity?: (identity: IdentityRecord) => void
  onEditIdentity?: (identity: IdentityRecord) => void
  onShareIdentity?: (identity: IdentityRecord) => void
  onDeleteIdentity?: (identity: IdentityRecord) => void
}

function formatDate(value?: string | null) {
  if (!value) {
    return 'Never'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Never'
  }
  return date.toLocaleString()
}

export function IdentityTable({
  identities,
  isLoading,
  templateNames,
  onViewIdentity,
  onEditIdentity,
  onShareIdentity,
  onDeleteIdentity,
}: IdentityTableProps) {
  const [sorting, setSorting] = useState<SortingState>([])

  const columns = useMemo<ColumnDef<IdentityRecord>[]>(
    () => [
      {
        accessorKey: 'name',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            type="button"
            className="-ml-3"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Identity
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => {
          const identity = row.original
          const templateName =
            identity.template_id && templateNames ? templateNames[identity.template_id] : undefined
          return (
            <div className="flex flex-col">
              <span className="text-sm font-medium text-foreground">{identity.name}</span>
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                {templateName ? (
                  <span className="inline-flex items-center gap-1">
                    <Layers className="h-3 w-3" />
                    {templateName}
                  </span>
                ) : null}
                {identity.description ? <span>{identity.description}</span> : null}
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: 'scope',
        header: () => 'Scope',
        cell: ({ row }) => <IdentityScopeBadge scope={row.original.scope} />,
        size: 120,
      },
      {
        id: 'usage',
        header: () => 'Usage',
        cell: ({ row }) => (
          <div className="space-y-1">
            <p className="text-sm font-semibold text-foreground">{row.original.usage_count}</p>
            <p className="text-xs text-muted-foreground">
              {row.original.connection_count} connection
              {row.original.connection_count === 1 ? '' : 's'}
            </p>
          </div>
        ),
        size: 140,
      },
      {
        accessorKey: 'last_used_at',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Last used
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {formatDate(row.original.last_used_at)}
          </span>
        ),
      },
      {
        accessorKey: 'updated_at',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Updated
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {formatDate(row.original.updated_at)}
          </span>
        ),
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">Actions</span>,
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex items-center justify-end gap-1">
            <Button
              type="button"
              size="sm"
              variant="ghost"
              aria-label={`View ${row.original.name}`}
              onClick={() => onViewIdentity?.(row.original)}
            >
              <Eye className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              aria-label={`Edit ${row.original.name}`}
              onClick={() => onEditIdentity?.(row.original)}
            >
              <PencilLine className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              aria-label={`Share ${row.original.name}`}
              onClick={() => onShareIdentity?.(row.original)}
            >
              <Share2 className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              aria-label={`Delete ${row.original.name}`}
              onClick={() => onDeleteIdentity?.(row.original)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        ),
        size: 150,
      },
    ],
    [onDeleteIdentity, onEditIdentity, onShareIdentity, onViewIdentity, templateNames]
  )

  const table = useReactTable({
    data: identities,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: { sorting },
    onSortingChange: setSorting,
  })

  if (isLoading) {
    return (
      <div className="space-y-3 rounded-lg border border-border bg-card p-6">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-6 w-full" />
        <Skeleton className="h-6 w-2/3" />
      </div>
    )
  }

  if (!identities.length) {
    return (
      <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/30 py-16 text-center">
        <p className="text-sm text-muted-foreground">
          No identities found for the selected filters.
        </p>
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card">
      <table className="min-w-full divide-y divide-border text-sm">
        <thead className="bg-muted/60">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className={cn(
                    'px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-muted-foreground',
                    header.column.id === 'actions' ? 'text-right' : ''
                  )}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className="divide-y divide-border">
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} className="hover:bg-muted/40">
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className="px-4 py-3 align-middle">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
