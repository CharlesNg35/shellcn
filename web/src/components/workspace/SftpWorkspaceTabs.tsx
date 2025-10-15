import { X, FileText, Folder } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
import type { WorkspaceTab } from '@/store/ssh-workspace-store'

interface SftpWorkspaceTabsProps {
  tabs: WorkspaceTab[]
  activeTabId: string
  onSelect: (tabId: string) => void
  onClose: (tabId: string) => void
}

export function SftpWorkspaceTabs({
  tabs,
  activeTabId,
  onSelect,
  onClose,
}: SftpWorkspaceTabsProps) {
  return (
    <div className="flex items-center gap-1 border-b border-border px-2">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId
        const isBrowser = tab.type === 'browser'
        const Icon = tab.type === 'browser' ? Folder : FileText

        return (
          <div
            key={tab.id}
            className={cn(
              'flex items-center rounded-md px-2 py-1 text-sm transition-colors',
              isActive ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted/40'
            )}
          >
            <Button
              variant="ghost"
              size="sm"
              className={cn('flex items-center gap-2 px-2', isActive && 'text-foreground')}
              onClick={() => onSelect(tab.id)}
            >
              <Icon className="h-4 w-4" aria-hidden />
              <span className="max-w-[160px] truncate">{tab.title}</span>
              {tab.dirty && <span className="text-primary">•</span>}
            </Button>
            {!isBrowser && (
              <Button
                variant="ghost"
                size="icon"
                className="ml-1 h-6 w-6 text-muted-foreground hover:text-foreground"
                onClick={() => onClose(tab.id)}
                aria-label={`Close ${tab.title}`}
              >
                <X className="h-3 w-3" aria-hidden />
              </Button>
            )}
          </div>
        )
      })}
    </div>
  )
}
