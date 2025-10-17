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
    <div className="flex flex-col gap-2 rounded-lg border border-border bg-card/50 p-3 shadow-sm">
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={onNavigateUp}
          disabled={isRootPath}
          className="h-7 text-xs"
        >
          {navigateUpLabel ?? 'Up'}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={onNavigateHome}
          disabled={isRootPath}
          className="h-7 text-xs"
        >
          {navigateHomeLabel ?? 'Home'}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={onRefresh}
          disabled={isLoading}
          className="h-7 text-xs"
        >
          {refreshLabel ?? 'Refresh'}
        </Button>
        <div className="ml-auto flex items-center gap-2">
          <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Checkbox
              checked={showHidden}
              onCheckedChange={(checked) => onToggleHidden(Boolean(checked))}
            />
            Show hidden
          </label>
          {uploadControl}
        </div>
      </div>

      <form className="flex items-center gap-2" onSubmit={onSubmitPath}>
        <label className="text-xs font-medium text-muted-foreground" htmlFor="sftp-path">
          Path
        </label>
        <Input
          id="sftp-path"
          value={pathInput}
          onChange={(event) => onPathInputChange(event.target.value)}
          className="h-7 flex-1 text-xs"
          autoComplete="off"
        />
        <Button type="submit" size="sm" variant="secondary" className="h-7 text-xs">
          Go
        </Button>
      </form>
    </div>
  )
}
