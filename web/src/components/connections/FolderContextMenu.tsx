import { useCallback, useState } from 'react'
import { MoreVertical, Pencil, Trash2 } from 'lucide-react'
import type { ConnectionFolderSummary } from '@/types/connections'
import { cn } from '@/lib/utils/cn'

interface FolderContextMenuProps {
  folder: ConnectionFolderSummary
  disabled?: boolean
  canEdit?: boolean
  canDelete?: boolean
  onEdit: (folder: ConnectionFolderSummary) => void
  onDelete: () => void
}

export function FolderContextMenu({
  folder,
  disabled,
  canEdit,
  canDelete,
  onEdit,
  onDelete,
}: FolderContextMenuProps) {
  const [open, setOpen] = useState(false)

  const close = useCallback(() => setOpen(false), [])

  const allowEdit = canEdit ?? true
  const allowDelete = canDelete ?? true
  const disableToggle = disabled ?? (!allowEdit && !allowDelete)

  const handleEdit = useCallback(() => {
    if (!allowEdit) {
      return
    }
    onEdit(folder)
    close()
  }, [allowEdit, folder, onEdit, close])

  const handleDelete = useCallback(() => {
    if (!allowDelete) {
      return
    }
    onDelete()
    close()
  }, [allowDelete, onDelete, close])

  return (
    <div className="relative">
      <button
        type="button"
        disabled={disableToggle}
        onClick={() => setOpen((value) => !value)}
        className={cn(
          'rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground',
          disableToggle && 'opacity-50'
        )}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Folder actions for ${folder.name}`}
      >
        <MoreVertical className="h-4 w-4" />
      </button>

      {open ? (
        <>
          <div className="fixed inset-0 z-10" onClick={close} />
          <div className="absolute right-0 top-8 z-20 w-48 rounded-md border border-border bg-popover p-1 shadow-lg">
            <button
              type="button"
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-3 py-2 text-sm text-foreground hover:bg-accent',
                !allowEdit && 'cursor-not-allowed opacity-60 hover:bg-transparent'
              )}
              onClick={handleEdit}
              disabled={!allowEdit}
              aria-disabled={!allowEdit}
            >
              <Pencil className="h-4 w-4" />
              Edit
            </button>

            <button
              type="button"
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-3 py-2 text-sm text-destructive hover:bg-destructive/10',
                !allowDelete && 'cursor-not-allowed opacity-60 hover:bg-transparent'
              )}
              onClick={handleDelete}
              disabled={!allowDelete}
              aria-disabled={!allowDelete}
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          </div>
        </>
      ) : null}
    </div>
  )
}
