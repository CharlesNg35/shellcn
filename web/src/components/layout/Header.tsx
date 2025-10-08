import { Bell, ChevronDown, LogOut, Settings, User } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import { cn } from '@/lib/utils/cn'

export function Header() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setUserMenuOpen(false)
      }
    }

    if (userMenuOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [userMenuOpen])

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  const getUserInitials = () => {
    if (user?.first_name && user?.last_name) {
      return `${user.first_name[0]}${user.last_name[0]}`.toUpperCase()
    }
    if (user?.username) {
      return user.username.slice(0, 2).toUpperCase()
    }
    return 'U'
  }

  const getUserDisplayName = () => {
    if (user?.first_name && user?.last_name) {
      return `${user.first_name} ${user.last_name}`
    }
    return user?.username || 'User'
  }

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border bg-card px-6 shadow-sm">
      {/* Left side - Breadcrumbs or page title */}
      <div className="flex items-center gap-4">
        <h2 className="text-lg font-semibold text-foreground">Dashboard</h2>
      </div>

      {/* Right side - Notifications and user menu */}
      <div className="flex items-center gap-4">
        {/* Notifications */}
        <button
          className="relative rounded-lg p-2 text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label="Notifications"
        >
          <Bell className="h-5 w-5" />
          {/* Notification badge */}
          <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-destructive" />
        </button>

        {/* User menu */}
        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setUserMenuOpen(!userMenuOpen)}
            className="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-muted"
          >
            {/* Avatar */}
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
              {getUserInitials()}
            </div>

            {/* User info */}
            <div className="hidden text-left sm:block">
              <div className="text-sm font-medium text-foreground">{getUserDisplayName()}</div>
              <div className="text-xs text-muted-foreground">{user?.email}</div>
            </div>

            <ChevronDown
              className={cn(
                'h-4 w-4 text-muted-foreground transition-transform',
                userMenuOpen && 'rotate-180'
              )}
            />
          </button>

          {/* Dropdown menu */}
          {userMenuOpen && (
            <div className="absolute right-0 mt-2 w-56 rounded-lg border border-border bg-popover shadow-lg">
              <div className="border-b border-border px-4 py-3">
                <div className="text-sm font-medium text-popover-foreground">
                  {getUserDisplayName()}
                </div>
                <div className="text-xs text-muted-foreground">{user?.email}</div>
                {user?.is_root && (
                  <div className="mt-1">
                    <span className="inline-flex items-center rounded-md bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
                      Root Admin
                    </span>
                  </div>
                )}
              </div>

              <div className="py-1">
                <Link
                  to="/profile"
                  className="flex items-center gap-3 px-4 py-2 text-sm text-popover-foreground hover:bg-muted"
                  onClick={() => setUserMenuOpen(false)}
                >
                  <User className="h-4 w-4" />
                  Profile
                </Link>
                <Link
                  to="/settings"
                  className="flex items-center gap-3 px-4 py-2 text-sm text-popover-foreground hover:bg-muted"
                  onClick={() => setUserMenuOpen(false)}
                >
                  <Settings className="h-4 w-4" />
                  Settings
                </Link>
              </div>

              <div className="border-t border-border py-1">
                <button
                  onClick={handleLogout}
                  className="flex w-full items-center gap-3 px-4 py-2 text-sm text-destructive hover:bg-muted"
                >
                  <LogOut className="h-4 w-4" />
                  Sign out
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
