import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Server,
  Monitor,
  Database,
  Container,
  Cloud,
  HardDrive,
  Users,
  Building2,
  Shield,
  Settings,
  FileText,
  Activity,
  Key,
  ChevronRight,
} from 'lucide-react'
import { cn } from '@/lib/utils/cn'
import logo from '@/assets/logo.svg'
import { APP_NAME } from '@/lib/constants'

interface NavItem {
  to: string
  icon: React.ElementType
  label: string
  permission?: string
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: 'Main',
    items: [
      { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
      { to: '/connections', icon: Server, label: 'All Connections' },
    ],
  },
  {
    label: 'Connections',
    items: [
      {
        to: '/ssh',
        icon: Server,
        label: 'SSH / Telnet',
      },
      {
        to: '/rdp',
        icon: Monitor,
        label: 'RDP',
      },
      {
        to: '/vnc',
        icon: Monitor,
        label: 'VNC',
      },
      {
        to: '/docker',
        icon: Container,
        label: 'Docker',
      },
      {
        to: '/kubernetes',
        icon: Cloud,
        label: 'Kubernetes',
      },
      {
        to: '/databases',
        icon: Database,
        label: 'Databases',
      },
      {
        to: '/proxmox',
        icon: HardDrive,
        label: 'Proxmox',
      },
    ],
  },
  {
    label: 'Settings',
    items: [
      {
        to: '/settings/identities',
        icon: Key,
        label: 'Identities',
      },
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
    ],
  },
]

export function Sidebar() {
  const hasPermission = () => {
    return true
  }

  return (
    <aside className="fixed left-0 top-0 z-40 hidden h-screen w-64 flex-col border-r border-border bg-background lg:flex">
      <div className="flex h-14 items-center border-b border-border px-6">
        <div className="flex items-center gap-2">
          <img src={logo} alt={APP_NAME} className="h-6 w-6" />
          <span className="text-sm font-semibold">{APP_NAME}</span>
        </div>
      </div>

      <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-4 scrollbar-thin">
        {navGroups.map((group) => (
          <div key={group.label} className="space-y-1">
            <h4 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {group.label}
            </h4>
            {group.items.map((item) => {
              if (item.permission && !hasPermission()) {
                return null
              }

              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      'group flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-all',
                      isActive
                        ? 'bg-primary text-primary-foreground shadow-sm'
                        : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                    )
                  }
                >
                  {({ isActive }) => (
                    <>
                      <item.icon
                        className={cn(
                          'h-4 w-4 shrink-0 transition-transform group-hover:scale-110',
                          isActive && 'scale-110'
                        )}
                      />
                      <span>{item.label}</span>
                      {isActive && <ChevronRight className="ml-auto h-4 w-4 opacity-50" />}
                    </>
                  )}
                </NavLink>
              )
            })}
          </div>
        ))}
      </nav>
    </aside>
  )
}
