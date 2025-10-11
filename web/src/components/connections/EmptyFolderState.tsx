import { Folder, Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'

interface EmptyFolderStateProps {
  canManageFolders: boolean
  onCreateFolder: () => void
}

export function EmptyFolderState({ canManageFolders, onCreateFolder }: EmptyFolderStateProps) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 rounded-lg border border-dashed border-border/70 bg-muted/30 p-6 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Folder className="h-6 w-6" />
      </div>
      <div className="space-y-1">
        <h3 className="text-base font-semibold text-foreground">No folders yet</h3>
        <p className="text-sm text-muted-foreground">
          Organize your connections into folders for faster navigation.
        </p>
      </div>
      {canManageFolders ? (
        <Button onClick={onCreateFolder} size="sm" className="shadow-sm">
          <Plus className="mr-2 h-4 w-4" />
          Create Folder
        </Button>
      ) : null}
    </div>
  )
}
