import { ShieldCheck, ShieldOff, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PERMISSIONS } from '@/constants/permissions'

interface UserBulkActionsBarProps {
  selectedCount: number
  onActivate: () => void
  onDeactivate: () => void
  onDelete: () => void
  isProcessing?: boolean
}

export function UserBulkActionsBar({
  selectedCount,
  onActivate,
  onDeactivate,
  onDelete,
  isProcessing,
}: UserBulkActionsBarProps) {
  if (!selectedCount) {
    return null
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border/70 bg-muted/40 px-4 py-3">
      <div className="text-sm font-medium text-muted-foreground">
        {selectedCount} user{selectedCount === 1 ? '' : 's'} selected
      </div>
      <div className="flex flex-wrap gap-2">
        <PermissionGuard permission={PERMISSIONS.USER.EDIT}>
          <Button size="sm" variant="secondary" onClick={onActivate} disabled={isProcessing}>
            <ShieldCheck className="mr-2 h-4 w-4" /> Activate
          </Button>
          <Button size="sm" variant="outline" onClick={onDeactivate} disabled={isProcessing}>
            <ShieldOff className="mr-2 h-4 w-4" /> Deactivate
          </Button>
        </PermissionGuard>
        <PermissionGuard permission={PERMISSIONS.USER.DELETE}>
          <Button size="sm" variant="destructive" onClick={onDelete} disabled={isProcessing}>
            <Trash2 className="mr-2 h-4 w-4" /> Delete
          </Button>
        </PermissionGuard>
      </div>
    </div>
  )
}
