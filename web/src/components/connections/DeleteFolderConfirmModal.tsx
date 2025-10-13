import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import type { ConnectionFolderNode } from '@/types/connections'

interface DeleteFolderConfirmModalProps {
  open: boolean
  folder: ConnectionFolderNode | null
  onClose: () => void
  onConfirm: () => Promise<void>
}

export function DeleteFolderConfirmModal({
  open,
  folder,
  onClose,
  onConfirm,
}: DeleteFolderConfirmModalProps) {
  const [confirmation, setConfirmation] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const folderName = folder?.folder.name ?? ''
  const connectionCount = folder?.connection_count ?? 0
  const isValid = confirmation.trim() === folderName && folderName.length > 0

  const handleSubmit = async () => {
    if (!isValid) {
      return
    }
    try {
      setIsSubmitting(true)
      await onConfirm()
      setConfirmation('')
      onClose()
    } finally {
      setIsSubmitting(false)
    }
  }

  const description = connectionCount
    ? `${connectionCount} connection${connectionCount === 1 ? '' : 's'} will be moved to "Unassigned".`
    : 'No connections will be moved.'

  return (
    <Modal
      open={open}
      onClose={() => {
        setConfirmation('')
        onClose()
      }}
      title="Delete folder?"
      description="Deleting a folder reassigns its connections to the Unassigned view."
    >
      <div className="space-y-5">
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="h-5 w-5 text-destructive" />
            <div className="space-y-1">
              <p className="text-sm font-semibold text-foreground">This action cannot be undone.</p>
              <p className="text-sm text-muted-foreground">{description}</p>
              <p className="text-xs text-muted-foreground">
                Type the folder name to confirm: <span className="font-medium">{folderName}</span>
              </p>
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <label htmlFor="delete-folder-name" className="text-sm font-medium text-foreground">
            Folder name
          </label>
          <input
            id="delete-folder-name"
            type="text"
            className="h-10 w-full rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-destructive"
            placeholder="Enter folder name to confirm"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>

        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setConfirmation('')
              onClose()
            }}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={handleSubmit}
            disabled={!isValid}
            loading={isSubmitting}
          >
            Delete Folder
          </Button>
        </div>
      </div>
    </Modal>
  )
}
