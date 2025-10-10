import { type ReactNode } from 'react'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'

interface PageContainerProps {
  title?: string
  description?: string
  action?: ReactNode
  children: ReactNode
  className?: string
  isLoading?: boolean
  error?: Error | null
  onRefresh?: () => void
  showRefresh?: boolean
}

export function PageContainer({
  title,
  description,
  action,
  children,
  className,
  isLoading,
  error,
  onRefresh,
  showRefresh = false,
}: PageContainerProps) {
  return (
    <div className={cn('space-y-6', className)}>
      {(title || description || action) && (
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex-1 space-y-1">
            {title && (
              <div className="flex items-center gap-3">
                <h1 className="text-3xl font-bold tracking-tight text-foreground">{title}</h1>
                {(showRefresh || onRefresh) && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={onRefresh}
                    disabled={isLoading}
                    className="h-8 w-8 p-0"
                  >
                    <RefreshCw className={cn('h-4 w-4', isLoading && 'animate-spin')} />
                    <span className="sr-only">Refresh</span>
                  </Button>
                )}
              </div>
            )}
            {description && <p className="text-sm text-muted-foreground">{description}</p>}
          </div>
          {action && <div className="flex items-center gap-2">{action}</div>}
        </header>
      )}

      {error ? (
        <ErrorState error={error} onRetry={onRefresh} />
      ) : isLoading ? (
        <LoadingState />
      ) : (
        children
      )}
    </div>
  )
}

function LoadingState() {
  return (
    <div className="flex min-h-[400px] items-center justify-center rounded-lg border border-dashed border-border bg-muted/20">
      <div className="flex items-center gap-3 text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" />
        <span className="text-sm font-medium">Loading...</span>
      </div>
    </div>
  )
}

interface ErrorStateProps {
  error: Error
  onRetry?: () => void
}

function ErrorState({ error, onRetry }: ErrorStateProps) {
  return (
    <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 rounded-lg border border-destructive/40 bg-destructive/10 px-6 py-12 text-center">
      <div className="space-y-2">
        <h3 className="text-lg font-semibold text-destructive">Something went wrong</h3>
        <p className="max-w-md text-sm text-destructive/90">
          {error.message || 'An unexpected error occurred. Please try again.'}
        </p>
      </div>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw className="mr-2 h-4 w-4" />
          Try Again
        </Button>
      )}
    </div>
  )
}

interface PageSectionProps {
  title?: string
  description?: string
  action?: ReactNode
  children: ReactNode
  className?: string
}

export function PageSection({ title, description, action, children, className }: PageSectionProps) {
  return (
    <section className={cn('space-y-4', className)}>
      {(title || description || action) && (
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-1">
            {title && <h2 className="text-xl font-semibold tracking-tight">{title}</h2>}
            {description && <p className="text-sm text-muted-foreground">{description}</p>}
          </div>
          {action && <div className="flex items-center gap-2">{action}</div>}
        </div>
      )}
      {children}
    </section>
  )
}
