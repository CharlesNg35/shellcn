import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Building2,
  Shield,
  Settings,
  FileText,
  Activity,
  Key,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import { useState } from 'react'
import { cn } from '@/lib/utils/cn'
import logo from '@/assets/logo.svg'
import { APP_NAME } from '@/lib/constants'

interface NavItem {
  to: string
  icon: React.ElementType
  label: string
  permission?: string
}

const navItems: NavItem[] = [
  { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
  {
    to: '/settings/users',
    icon: Users,
    label: 'Users',
    permission: 'user.view',
  },
  {
    to: '/settings/organizations',
    icon: Building2,
    label: 'Organizations',
    permission: 'org.view',
  },
  {
    to: '/settings/teams',
    icon: Users,
    label: 'Teams',
    permission: 'org.view',
  },
  {
    to: '/settings/permissions',
    icon: Shield,
    label: 'Permissions',
    permission: 'permission.view',
  },
  {
    to: '/settings/auth-providers',
    icon: Key,
    label: 'Auth Providers',
    permission: 'permission.manage',
  },
  {
    to: '/settings/sessions',
    icon: Activity,
    label: 'Sessions',
  },
  {
    to: '/settings/audit',
    icon: FileText,
    label: 'Audit Logs',
    permission: 'audit.view',
  },
  {
    to: '/settings/security',
    icon: Settings,
    label: 'Security',
  },
]

export function Sidebar() {
  const [collapsed, setCollapsed] = useState(false)

  // TODO: Implement permission checking with usePermissions hook
  // const { hasPermission } = usePermissions()

  const hasPermission = (_permission: string) => {
    // Placeholder - always return true for now
    return true
  }

  return (
    <aside
      className={cn(
        'fixed left-0 top-0 z-40 h-screen border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-all duration-300',
        collapsed ? 'w-16' : 'w-64'
      )}
    >
      {/* Logo and collapse button */}
      <div className="flex h-16 items-center justify-between border-b border-sidebar-border px-4">
        {!collapsed && (
          <div className="flex items-center gap-3">
            <img src={logo} alt={APP_NAME} className="h-8 w-8" />
            <span className="text-lg font-semibold">{APP_NAME}</span>
          </div>
        )}
        {collapsed && (
          <div className="flex w-full justify-center">
            <img src={logo} alt={APP_NAME} className="h-8 w-8" />
          </div>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className={cn(
            'rounded-md p-1.5 text-sidebar-foreground/60 hover:bg-sidebar-accent/10 hover:text-sidebar-foreground',
            collapsed && 'hidden'
          )}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <ChevronRight className="h-5 w-5" /> : <ChevronLeft className="h-5 w-5" />}
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 overflow-y-auto p-3 scrollbar-thin">
        {navItems.map((item) => {
          // Hide items if user doesn't have permission
          if (item.permission && !hasPermission(item.permission)) {
            return null
          }

          return (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  'hover:bg-sidebar-accent/10 hover:text-sidebar-accent',
                  isActive
                    ? 'bg-sidebar-primary text-sidebar-primary-foreground'
                    : 'text-sidebar-foreground/80',
                  collapsed && 'justify-center'
                )
              }
              title={collapsed ? item.label : undefined}
            >
              <item.icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span>{item.label}</span>}
            </NavLink>
          )
        })}
      </nav>

      {/* Collapse button when sidebar is collapsed */}
      {collapsed && (
        <div className="border-t border-sidebar-border p-3">
          <button
            onClick={() => setCollapsed(false)}
            className="flex w-full items-center justify-center rounded-lg p-2.5 text-sidebar-foreground/60 hover:bg-sidebar-accent/10 hover:text-sidebar-foreground"
            aria-label="Expand sidebar"
          >
            <ChevronRight className="h-5 w-5" />
          </button>
        </div>
      )}
    </aside>
  )
}
