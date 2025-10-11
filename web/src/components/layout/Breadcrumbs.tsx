import { useMemo } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { ChevronRight, Home } from 'lucide-react'
import { getBreadcrumbItems } from '@/lib/navigation'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { cn } from '@/lib/utils/cn'

interface BreadcrumbsProps {
  className?: string
}

export function Breadcrumbs({ className }: BreadcrumbsProps) {
  const location = useLocation()
  const { overrides } = useBreadcrumb()

  const crumbs = useMemo(() => {
    const items = getBreadcrumbItems(location.pathname)
    // Apply overrides to crumbs
    return items.map((item) => ({
      ...item,
      label: overrides[item.path] || item.label,
    }))
  }, [location.pathname, overrides])

  if (!crumbs.length) {
    return null
  }

  return (
    <nav aria-label="Breadcrumb" className={cn('flex items-center gap-2 text-sm', className)}>
      <Link
        to="/dashboard"
        className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground"
      >
        <Home className="h-4 w-4" />
        <span className="sr-only">Dashboard</span>
      </Link>
      {crumbs.map((crumb, index) => {
        const isLast = index === crumbs.length - 1
        return (
          <div key={`${crumb.path}-${index}`} className="flex items-center gap-2">
            <ChevronRight className="h-3 w-3 text-muted-foreground" />
            {isLast ? (
              <span className="font-medium text-foreground">{crumb.label}</span>
            ) : (
              <Link
                to={crumb.path}
                className="text-muted-foreground transition hover:text-foreground"
              >
                {crumb.label}
              </Link>
            )}
          </div>
        )
      })}
    </nav>
  )
}
