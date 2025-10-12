interface RouteLoaderProps {
  /**
   * Optional message displayed under the spinner.
   */
  message?: string
}

export function RouteLoader({ message = 'Loading...' }: RouteLoaderProps) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-3">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        <p className="text-sm text-muted-foreground">{message}</p>
      </div>
    </div>
  )
}
