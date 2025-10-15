import type { FormEvent, ReactNode } from 'react'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { Input } from '@/components/ui/Input'

interface FileManagerToolbarProps {
  isRootPath: boolean
  isLoading: boolean
  showHidden: boolean
  onToggleHidden: (next: boolean) => void
  onNavigateUp: () => void
  onNavigateHome: () => void
  onRefresh: () => void
  pathInput: string
  onPathInputChange: (value: string) => void
  onSubmitPath: (event: FormEvent<HTMLFormElement>) => void
  uploadControl?: ReactNode
  navigateUpLabel?: ReactNode
  navigateHomeLabel?: ReactNode
  refreshLabel?: ReactNode
}

export function FileManagerToolbar({
  isRootPath,
  isLoading,
  showHidden,
  onToggleHidden,
  onNavigateUp,
  onNavigateHome,
  onRefresh,
  pathInput,
  onPathInputChange,
  onSubmitPath,
  uploadControl,
  navigateUpLabel,
  navigateHomeLabel,
  refreshLabel,
}: FileManagerToolbarProps) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="ghost" size="sm" onClick={onNavigateUp} disabled={isRootPath}>
          {navigateUpLabel ?? 'Up'}
        </Button>
        <Button variant="ghost" size="sm" onClick={onNavigateHome} disabled={isRootPath}>
          {navigateHomeLabel ?? 'Home'}
        </Button>
        <Button variant="ghost" size="sm" onClick={onRefresh} disabled={isLoading}>
          {refreshLabel ?? 'Refresh'}
        </Button>
        <div className="ml-auto flex items-center gap-2">
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <Checkbox
              checked={showHidden}
              onCheckedChange={(checked) => onToggleHidden(Boolean(checked))}
            />
            Show hidden files
          </label>
          {uploadControl}
        </div>
      </div>

      <form className="flex items-center gap-3" onSubmit={onSubmitPath}>
        <label
          className="text-xs font-semibold uppercase text-muted-foreground"
          htmlFor="sftp-path"
        >
          Current path
        </label>
        <Input
          id="sftp-path"
          value={pathInput}
          onChange={(event) => onPathInputChange(event.target.value)}
          className="flex-1"
          autoComplete="off"
        />
        <Button type="submit" size="sm" variant="secondary">
          Go
        </Button>
      </form>
    </div>
  )
}
