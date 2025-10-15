import { Fragment } from 'react'
import {
  LayoutGrid,
  File as FileIcon,
  Maximize2,
  Minimize2,
  Command as CommandIcon,
  ExternalLink,
  Loader2,
  Wand2,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/DropdownMenu'

interface SnippetGroup {
  label: string
  snippets: Array<{ id: string; name: string; description?: string }>
}

interface SshWorkspaceToolbarProps {
  layoutColumns: number
  layoutOptions: number[]
  onLayoutChange: (columns: number) => void
  snippetGroups: SnippetGroup[]
  loadingSnippets: boolean
  disabledSnippets: boolean
  onExecuteSnippet: (snippetId: string) => void
  onOpenFileManager: () => void
  showFileManagerButton: boolean
  isFullscreen: boolean
  onToggleFullscreen: () => void
  onOpenCommandPalette: () => void
  onOpenNewWindow: () => void
  snippetsAvailable: boolean
  showSnippetsButton: boolean
}

export function SshWorkspaceToolbar({
  layoutColumns,
  layoutOptions,
  onLayoutChange,
  snippetGroups,
  loadingSnippets,
  disabledSnippets,
  onExecuteSnippet,
  onOpenFileManager,
  showFileManagerButton,
  isFullscreen,
  onToggleFullscreen,
  onOpenCommandPalette,
  onOpenNewWindow,
  snippetsAvailable,
  showSnippetsButton,
}: SshWorkspaceToolbarProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 bg-muted/30 px-3 py-2">
      <div className="flex items-center gap-2">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Change layout">
              <LayoutGrid className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuLabel>Layout columns</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {layoutOptions.map((option) => {
              const active = option === layoutColumns
              return (
                <DropdownMenuItem
                  key={option}
                  onSelect={() => onLayoutChange(option)}
                  className="flex items-center justify-between"
                >
                  <span>
                    {option} column{option > 1 ? 's' : ''}
                  </span>
                  {active && <span className="text-xs text-muted-foreground">Active</span>}
                </DropdownMenuItem>
              )
            })}
          </DropdownMenuContent>
        </DropdownMenu>

        {showSnippetsButton && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="flex items-center gap-2"
                disabled={disabledSnippets}
              >
                {loadingSnippets ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Wand2 className="h-4 w-4" />
                )}
                Snippets
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-72" align="start">
              {loadingSnippets ? (
                <div className="px-3 py-2 text-sm text-muted-foreground">Loading snippets…</div>
              ) : !snippetsAvailable ? (
                <div className="px-3 py-2 text-sm text-muted-foreground">No snippets available</div>
              ) : (
                snippetGroups.map((group) => (
                  <Fragment key={group.label}>
                    <DropdownMenuLabel>{group.label}</DropdownMenuLabel>
                    {group.snippets.map((snippet) => (
                      <DropdownMenuItem
                        key={snippet.id}
                        onSelect={() => onExecuteSnippet(snippet.id)}
                        className="flex flex-col items-start gap-0.5"
                      >
                        <span className="text-sm font-medium">{snippet.name}</span>
                        {snippet.description && (
                          <span className="text-xs text-muted-foreground">
                            {snippet.description}
                          </span>
                        )}
                      </DropdownMenuItem>
                    ))}
                    <DropdownMenuSeparator />
                  </Fragment>
                ))
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}

        {showFileManagerButton && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onOpenFileManager}
            className="flex items-center gap-2"
          >
            <FileIcon className="h-4 w-4" />
            File Manager
          </Button>
        )}
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={onToggleFullscreen}
          className="flex items-center gap-2"
        >
          {isFullscreen ? (
            <>
              <Minimize2 className="h-4 w-4" />
              Exit Fullscreen
            </>
          ) : (
            <>
              <Maximize2 className="h-4 w-4" />
              Fullscreen
            </>
          )}
        </Button>

        <Button
          variant="ghost"
          size="icon"
          onClick={onOpenCommandPalette}
          aria-label="Open command palette"
        >
          <CommandIcon className="h-4 w-4" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          onClick={onOpenNewWindow}
          aria-label="Open workspace in new window"
        >
          <ExternalLink className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}

export type { SnippetGroup as SshSnippetGroup }

export default SshWorkspaceToolbar
