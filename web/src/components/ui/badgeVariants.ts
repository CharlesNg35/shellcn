import { cva } from 'class-variance-authority'

export const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground shadow-sm hover:opacity-90',
        secondary:
          'border-transparent bg-secondary text-secondary-foreground shadow-sm hover:opacity-90',
        destructive:
          'border-transparent bg-destructive text-destructive-foreground shadow-sm hover:opacity-90',
        success: 'border-transparent bg-accent text-accent-foreground shadow-sm hover:opacity-90',
        outline: 'text-foreground border-border',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)
