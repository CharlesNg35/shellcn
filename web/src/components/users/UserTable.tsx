import { useEffect, useMemo, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type RowSelectionState,
  type SortingState,
  useReactTable,
} from '@tanstack/react-table'
import {
  ArrowUpDown,
  Eye,
  Loader2,
  MoreHorizontal,
  PencilLine,
  ShieldAlert,
  UserCog,
} from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import type { ApiMeta } from '@/types/api'
import type { UserRecord } from '@/types/users'
import { cn } from '@/lib/utils/cn'
import { PERMISSIONS } from '@/constants/permissions'

interface UserTableProps {
  users: UserRecord[]
  meta?: ApiMeta
  page: number
  perPage: number
  isLoading?: boolean
  onPageChange: (page: number) => void
  onSelectionChange?: (selectedIds: string[]) => void
  onViewUser?: (user: UserRecord) => void
  onEditUser?: (user: UserRecord) => void
}

export function UserTable({
  users,
  meta,
  page,
  perPage,
  isLoading,
  onPageChange,
  onSelectionChange,
  onViewUser,
  onEditUser,
}: UserTableProps) {
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [sorting, setSorting] = useState<SortingState>([])

  const columns = useMemo<ColumnDef<UserRecord>[]>(
    () => [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            indeterminate={table.getIsSomePageRowsSelected()}
            onCheckedChange={(checked) => table.toggleAllPageRowsSelected(!!checked)}
            disabled={!table.getRowModel().rows.length}
            aria-label="Select all users"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(checked) => row.toggleSelected(!!checked)}
            aria-label={`Select ${row.original.username}`}
          />
        ),
        enableSorting: false,
        size: 30,
      },
      {
        accessorKey: 'username',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            className="-ml-3"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Username
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium text-sm text-foreground">{row.original.username}</span>
            <span className="text-xs text-muted-foreground">{row.original.email}</span>
          </div>
        ),
      },
      {
        accessorKey: 'roles',
        header: () => 'Roles',
        cell: ({ row }) => {
          const roles = row.original.roles ?? []
          if (!roles.length) {
            return <span className="text-xs text-muted-foreground">No roles</span>
          }
          return (
            <div className="flex flex-wrap gap-1">
              {roles.slice(0, 3).map((role) => (
                <Badge key={role.id} variant="outline" className="text-xs">
                  {role.name}
                </Badge>
              ))}
              {roles.length > 3 ? (
                <Badge variant="secondary" className="text-xs">
                  +{roles.length - 3}
                </Badge>
              ) : null}
            </div>
          )
        },
      },
      {
        accessorKey: 'auth_provider',
        header: () => 'Provider',
        cell: ({ row }) => {
          const provider = row.original.auth_provider?.toUpperCase() ?? 'LOCAL'
          const variant = provider === 'LOCAL' ? 'outline' : 'secondary'
          return (
            <Badge variant={variant} className="text-xs">
              {provider}
            </Badge>
          )
        },
        size: 90,
      },
      {
        accessorKey: 'is_active',
        header: () => 'Status',
        cell: ({ row }) => (
          <Badge variant={row.original.is_active ? 'success' : 'secondary'}>
            {row.original.is_active ? 'Active' : 'Inactive'}
          </Badge>
        ),
      },
      {
        accessorKey: 'last_login_at',
        header: ({ column }) => (
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
          >
            Last Login
            <ArrowUpDown className="ml-2 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }) => {
          const value = row.original.last_login_at
          if (!value) {
            return <span className="text-xs text-muted-foreground">Never</span>
          }
          const date = new Date(value)
          return <span className="text-xs text-muted-foreground">{date.toLocaleString()}</span>
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
              onClick={() => onViewUser?.(row.original)}
            >
              <Eye className="h-4 w-4" />
            </Button>
            <PermissionGuard permission={PERMISSIONS.USER.EDIT}>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onEditUser?.(row.original)}
                aria-label={
                  row.original.auth_provider && row.original.auth_provider !== 'local'
                    ? `Manage access for ${row.original.username}`
                    : `Edit ${row.original.username}`
                }
              >
                {row.original.auth_provider && row.original.auth_provider !== 'local' ? (
                  <UserCog className="h-4 w-4" />
                ) : (
                  <PencilLine className="h-4 w-4" />
                )}
              </Button>
            </PermissionGuard>
            {row.original.is_root ? (
              <ShieldAlert className="h-4 w-4 text-muted-foreground" />
            ) : (
              <PermissionGuard permission={PERMISSIONS.USER.EDIT}>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => onViewUser?.(row.original)}
                >
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </PermissionGuard>
            )}
          </div>
        ),
        enableSorting: false,
        size: 80,
      },
    ],
    [onEditUser, onViewUser]
  )

  const table = useReactTable({
    data: users,
    columns,
    state: {
      rowSelection,
      sorting,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => row.id,
  })

  useEffect(() => {
    if (!onSelectionChange) {
      return
    }
    const selected = table.getSelectedRowModel().rows.map((row) => row.original.id)
    onSelectionChange(selected)
  }, [onSelectionChange, table, rowSelection])

  const total = meta?.total ?? users.length
  const totalPages = meta?.total_pages ?? Math.max(1, Math.ceil(total / perPage))

  const handlePrevious = () => {
    if (page > 1) {
      onPageChange(page - 1)
    }
  }

  const handleNext = () => {
    if (page < totalPages) {
      onPageChange(page + 1)
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center rounded-lg border border-border bg-card py-10">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!users.length) {
    return (
      <div className="rounded-lg border border-dashed border-border/70 bg-muted/30 p-8 text-center">
        <p className="text-sm text-muted-foreground">No users found for the current filters.</p>
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card shadow-sm">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[720px]">
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
                className={cn(
                  'border-t border-border/60',
                  row.getIsSelected() ? 'bg-primary/5 hover:bg-primary/10' : 'hover:bg-muted/40'
                )}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    className={cn(
                      'px-4 py-3 text-sm align-middle',
                      cell.column.id === 'username' ? 'whitespace-nowrap' : ''
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/60 bg-muted/20 px-4 py-3 text-sm">
        <div className="text-muted-foreground">
          Showing {(page - 1) * perPage + 1}–{Math.min(page * perPage, total)} of {total}
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handlePrevious}
            disabled={page === 1}
          >
            Previous
          </Button>
          <div className="text-xs font-medium text-muted-foreground">
            Page {page} of {totalPages}
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleNext}
            disabled={page >= totalPages}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  )
}
