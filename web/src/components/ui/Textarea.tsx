import * as React from 'react'
import { cn } from '@/lib/utils/cn'

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string
  error?: string
  helpText?: string
}

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, label, error, helpText, ...props }, ref) => {
    const textarea = (
      <textarea
        ref={ref}
        className={cn(
          'flex min-h-[120px] w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background transition-all placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
          error && 'border-destructive focus-visible:ring-destructive',
          className
        )}
        aria-invalid={Boolean(error) || undefined}
        {...props}
      />
    )

    return (
      <div className="w-full">
        {label ? (
          <label className="block text-sm font-medium text-foreground">
            {label}
            <span className="mt-2 block">{textarea}</span>
          </label>
        ) : (
          textarea
        )}
        {error && <p className="mt-1.5 text-sm text-destructive">{error}</p>}
        {helpText && !error && <p className="mt-1.5 text-sm text-muted-foreground">{helpText}</p>}
      </div>
    )
  }
)
Textarea.displayName = 'Textarea'

export { Textarea }
