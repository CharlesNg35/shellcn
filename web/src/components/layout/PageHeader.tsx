import type { ReactNode } from 'react'
import { cn } from '@/lib/utils/cn'

interface PageHeaderProps {
  title: string
  description?: string
  action?: ReactNode
  badge?: ReactNode
  className?: string
}

/**
 * PageHeader component for consistent page headers across the application.
 *
 * Usage:
 * ```tsx
 * <PageHeader
 *   title="Users"
 *   description="Manage platform users, activation status, and administrative privileges"
 *   action={
 *     <Button>
 *       <Plus className="mr-1 h-4 w-4" />
 *       Create User
 *     </Button>
 *   }
 * />
 * ```
 */
export function PageHeader({ title, description, action, badge, className }: PageHeaderProps) {
  return (
    <div className={cn('flex flex-wrap items-start justify-between gap-4', className)}>
      <div className="flex-1 space-y-1">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-bold tracking-tight text-foreground">{title}</h1>
          {badge}
        </div>
        {description && (
          <p className="max-w-2xl text-sm text-muted-foreground leading-relaxed">{description}</p>
        )}
      </div>
      {action && <div className="flex flex-wrap gap-2">{action}</div>}
    </div>
  )
}
