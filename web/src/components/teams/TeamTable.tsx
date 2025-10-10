import { useMemo, useState, type ReactNode } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  useReactTable,
} from '@tanstack/react-table'
import { ArrowUpDown, Eye, Loader2, PencilLine, Trash2, Users } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import type { TeamRecord } from '@/types/teams'
import { PERMISSIONS } from '@/constants/permissions'

interface TeamTableProps {
  teams: TeamRecord[]
  isLoading?: boolean
  memberCounts?: Record<string, number | undefined>
  onSelectTeam: (teamId: string) => void
  onEditTeam?: (team: TeamRecord) => void
  onDeleteTeam?: (team: TeamRecord) => void
  emptyAction?: ReactNode
}

export function TeamTable({
  teams,
  isLoading,
  memberCounts,
  onSelectTeam,
  onEditTeam,
  onDeleteTeam,
  emptyAction,
}: TeamTableProps) {
  const [sorting, setSorting] = useState<SortingState>([])

  const columns = useMemo<ColumnDef<TeamRecord>[]>(
    () => [
      {
        accessorKey: 'name',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            className="-ml-3"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Team Name
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium text-sm text-foreground">{row.original.name}</span>
            {row.original.description ? (
              <span className="text-xs text-muted-foreground line-clamp-1">
                {row.original.description}
              </span>
            ) : null}
          </div>
        ),
      },
      {
        id: 'members',
        header: () => 'Members',
        cell: ({ row }) => {
          const memberCount = memberCounts?.[row.original.id] ?? row.original.members?.length
          return (
            <Badge
              variant="secondary"
              className="flex items-center gap-1 w-fit text-xs font-medium"
            >
              <Users className="h-3 w-3" />
              {typeof memberCount === 'number' ? memberCount : 0}
            </Badge>
          )
        },
        enableSorting: false,
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Created
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => {
          const value = row.original.created_at
          if (!value) {
            return <span className="text-xs text-muted-foreground">—</span>
          }
          const date = new Date(value)
          return <span className="text-xs text-muted-foreground">{date.toLocaleDateString()}</span>
        },
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => (
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => onSelectTeam(row.original.id)}
              aria-label={`View ${row.original.name}`}
            >
              <Eye className="h-4 w-4" />
            </Button>
            <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onEditTeam?.(row.original)}
                aria-label={`Edit ${row.original.name}`}
              >
                <PencilLine className="h-4 w-4" />
              </Button>
            </PermissionGuard>
            <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="text-destructive hover:text-destructive"
                onClick={() => onDeleteTeam?.(row.original)}
                aria-label={`Delete ${row.original.name}`}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </PermissionGuard>
          </div>
        ),
        enableSorting: false,
        size: 120,
      },
    ],
    [memberCounts, onSelectTeam, onEditTeam, onDeleteTeam]
  )

  const table = useReactTable({
    data: teams,
    columns,
    state: {
      sorting,
    },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => row.id,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center rounded-lg border border-border bg-card py-10">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!teams.length) {
    return (
      <EmptyState
        icon={Users}
        title="No teams created yet"
        description="Organize your users into teams to simplify permission management and access control."
        action={emptyAction}
      />
    )
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card shadow-sm">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-muted/50 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id} className="px-4 py-3 text-left">
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr
                key={row.id}
                className="border-t border-border/60 hover:bg-muted/40 cursor-pointer"
                onClick={() => onSelectTeam(row.original.id)}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    className="px-4 py-3 text-sm align-middle"
                    onClick={cell.column.id === 'actions' ? (e) => e.stopPropagation() : undefined}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
