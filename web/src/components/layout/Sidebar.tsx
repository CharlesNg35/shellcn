import { NavLink } from 'react-router-dom'
import { X } from 'lucide-react'
import logo from '@/assets/logo.svg'
import { APP_NAME } from '@/lib/constants'
import { NAVIGATION_GROUPS, type NavigationItem } from '@/lib/navigation'
import { cn } from '@/lib/utils/cn'
import { usePermissions } from '@/hooks/usePermissions'

interface SidebarProps {
  isOpen?: boolean
  onClose?: () => void
}

function canRenderItem(item: NavigationItem, hasPermission: (permission: string) => boolean) {
  if (!item.permission) {
    return true
  }
  return hasPermission(item.permission)
}

export function Sidebar({ isOpen = false, onClose }: SidebarProps) {
  const { hasPermission } = usePermissions()

  const navContent = (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center justify-between border-b border-border px-6">
        <div className="flex items-center gap-2">
          <img src={logo} alt={APP_NAME} className="h-6 w-6" />
          <span className="text-sm font-semibold">{APP_NAME}</span>
        </div>
        <button
          className="rounded-lg p-1 text-muted-foreground hover:bg-muted lg:hidden"
          onClick={onClose}
          aria-label="Close navigation"
          type="button"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-4 scrollbar-thin">
        {NAVIGATION_GROUPS.map((group) => {
          const visibleItems = group.items.filter((item) => canRenderItem(item, hasPermission))

          if (!visibleItems.length) {
            return null
          }

          return (
            <div key={group.label} className="space-y-1">
              <h4 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {group.label}
              </h4>
              {visibleItems.map((item) => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  onClick={onClose}
                  className={({ isActive }) =>
                    cn(
                      'group flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-all',
                      isActive
                        ? 'bg-primary text-primary-foreground shadow-sm'
                        : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                    )
                  }
                >
                  {item.icon ? (
                    <item.icon className="h-4 w-4 shrink-0 transition-transform group-hover:scale-110" />
                  ) : null}
                  <span>{item.label}</span>
                </NavLink>
              ))}
            </div>
          )
        })}
      </nav>
    </div>
  )

  return (
    <>
      <aside className="fixed left-0 top-0 z-40 hidden h-screen w-64 flex-col border-r border-border bg-background lg:flex">
        {navContent}
      </aside>

      <div
        className={cn(
          'lg:hidden',
          isOpen ? 'fixed inset-0 z-50 block' : 'pointer-events-none fixed inset-0 z-50 hidden'
        )}
      >
        <div className="absolute inset-0 bg-background/70 backdrop-blur-sm" onClick={onClose} />
        <aside className="absolute inset-y-0 left-0 h-full w-64 border-r border-border bg-background shadow-xl">
          {navContent}
        </aside>
      </div>
    </>
  )
}
