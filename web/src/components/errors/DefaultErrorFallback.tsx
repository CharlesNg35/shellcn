import { AlertTriangle, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/Button'

interface DefaultErrorFallbackProps {
  error: Error
  resetError: () => void
}

export function DefaultErrorFallback({ error, resetError }: DefaultErrorFallbackProps) {
  return (
    <div className="flex min-h-[600px] items-center justify-center p-6">
      <div className="w-full max-w-md space-y-6 text-center">
        <div className="flex justify-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-destructive/10">
            <AlertTriangle className="h-8 w-8 text-destructive" />
          </div>
        </div>

        <div className="space-y-2">
          <h1 className="text-2xl font-bold text-foreground">Something went wrong</h1>
          <p className="text-sm text-muted-foreground">
            An unexpected error occurred. This has been logged and we'll look into it.
          </p>
        </div>

        <details className="rounded-lg border border-border bg-muted/20 p-4 text-left">
          <summary className="cursor-pointer text-sm font-medium text-foreground">
            Error details
          </summary>
          <div className="mt-3 space-y-2">
            <div>
              <div className="text-xs font-medium text-muted-foreground">Message:</div>
              <code className="mt-1 block rounded bg-muted p-2 text-xs text-foreground">
                {error.message}
              </code>
            </div>
            {error.stack && (
              <div>
                <div className="text-xs font-medium text-muted-foreground">Stack trace:</div>
                <pre className="mt-1 max-h-40 overflow-auto rounded bg-muted p-2 text-xs text-foreground">
                  {error.stack}
                </pre>
              </div>
            )}
          </div>
        </details>

        <div className="flex flex-col gap-2 sm:flex-row sm:justify-center">
          <Button onClick={resetError} variant="default">
            <RefreshCw className="mr-2 h-4 w-4" />
            Try Again
          </Button>
          <Button onClick={() => (window.location.href = '/')} variant="outline">
            Go to Dashboard
          </Button>
        </div>
      </div>
    </div>
  )
}
