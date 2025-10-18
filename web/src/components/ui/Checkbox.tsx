import * as React from 'react'
import * as CheckboxPrimitive from '@radix-ui/react-checkbox'
import { Check, Minus } from 'lucide-react'
import { cn } from '@/lib/utils/cn'

export interface CheckboxProps
  extends Omit<
    React.ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root>,
    'checked' | 'onCheckedChange'
  > {
  checked?: boolean
  onCheckedChange?: (checked: boolean) => void
  indeterminate?: boolean
}

export const Checkbox = React.forwardRef<
  React.ElementRef<typeof CheckboxPrimitive.Root>,
  CheckboxProps
>(({ className, checked, onCheckedChange, indeterminate, ...props }, ref) => {
  // Convert indeterminate to Radix's "indeterminate" state
  const checkedState = indeterminate ? 'indeterminate' : checked

  return (
    <CheckboxPrimitive.Root
      ref={ref}
      className={cn(
        'peer flex h-4 w-4 shrink-0 items-center justify-center rounded border border-input bg-background shadow-xs transition-shadow',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground',
        'data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-primary-foreground',
        className
      )}
      checked={checkedState}
      onCheckedChange={(value) => {
        // Handle indeterminate state - convert back to boolean
        const isChecked = value === true || value === 'indeterminate'
        onCheckedChange?.(isChecked)
      }}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="flex items-center justify-center text-current">
        {indeterminate ? (
          <Minus className="h-3 w-3" />
        ) : (
          <Check className="h-3 w-3" strokeWidth={3} />
        )}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  )
})

Checkbox.displayName = 'Checkbox'
