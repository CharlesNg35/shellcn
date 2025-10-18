import { useEffect, useMemo, useState } from 'react'
import { Reorder, AnimatePresence } from 'framer-motion'
import { X } from 'lucide-react'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'
import { cn } from '@/lib/utils/cn'

interface SshWorkspaceTabsBarProps {
  tabs: WorkspaceTab[]
  activeTabId: string
  onTabSelect: (tabId: string) => void
  onTabClose: (tabId: string) => void
  onTabsReordered?: (orderedTabIds: string[]) => void
}

function shallowEqualTabs(a: WorkspaceTab[], b: WorkspaceTab[]) {
  if (a.length !== b.length) {
    return false
  }
  for (let index = 0; index < a.length; index += 1) {
    if (a[index]?.id !== b[index]?.id) {
      return false
    }
  }
  return true
}

export function SshWorkspaceTabsBar({
  tabs,
  activeTabId,
  onTabSelect,
  onTabClose,
  onTabsReordered,
}: SshWorkspaceTabsBarProps) {
  const tabMap = useMemo(() => {
    const map = new Map<string, WorkspaceTab>()
    tabs.forEach((tab) => map.set(tab.id, tab))
    return map
  }, [tabs])

  const [orderedTabs, setOrderedTabs] = useState(tabs)

  useEffect(() => {
    setOrderedTabs((previous) => {
      const existing = previous
        .map((tab) => tabMap.get(tab.id))
        .filter((tab): tab is WorkspaceTab => Boolean(tab))
      const missing = tabs.filter((tab) => !existing.some((item) => item.id === tab.id))
      const candidate = [...existing, ...missing]
      if (candidate.length !== tabs.length) {
        return tabs
      }
      if (!shallowEqualTabs(candidate, tabs)) {
        return tabs
      }
      return shallowEqualTabs(candidate, previous) ? previous : candidate
    })
  }, [tabMap, tabs])

  const handleReorder = (nextTabs: WorkspaceTab[]) => {
    setOrderedTabs(nextTabs)
    onTabsReordered?.(nextTabs.map((item) => item.id))
  }

  return (
    <Reorder.Group
      axis="x"
      className="flex items-center gap-1"
      values={orderedTabs}
      onReorder={handleReorder}
      as="div"
    >
      <AnimatePresence initial={false}>
        {orderedTabs.map((tab) => {
          const isActive = tab.id === activeTabId
          return (
            <Reorder.Item
              value={tab}
              key={tab.id}
              dragListener={orderedTabs.length > 1}
              data-testid={`workspace-tab-${tab.id}`}
              className="list-none"
              layout
              whileTap={{ scale: 0.98 }}
            >
              <button
                type="button"
                onClick={() => onTabSelect(tab.id)}
                className={cn(
                  'group flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground shadow'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )}
              >
                <span className="truncate" title={tab.title}>
                  {tab.title}
                </span>
                {tab.meta?.badge && (
                  <span className="rounded bg-muted-foreground/10 px-1.5 py-0.5 text-xs text-muted-foreground">
                    {tab.meta.badge}
                  </span>
                )}
                {tab.closable && (
                  <span
                    role="button"
                    tabIndex={0}
                    className="rounded-sm p-0.5 text-muted-foreground outline-none transition hover:bg-muted/60 hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring"
                    onClick={(event) => {
                      event.stopPropagation()
                      onTabClose(tab.id)
                    }}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault()
                        onTabClose(tab.id)
                      }
                    }}
                    aria-label={`Close ${tab.title}`}
                  >
                    <X className="h-3 w-3" aria-hidden />
                  </span>
                )}
              </button>
            </Reorder.Item>
          )
        })}
      </AnimatePresence>
    </Reorder.Group>
  )
}

export default SshWorkspaceTabsBar
