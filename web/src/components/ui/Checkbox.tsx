import { forwardRef, type InputHTMLAttributes } from 'react'
import { Check, Minus } from 'lucide-react'
import { cn } from '@/lib/utils/cn'

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  indeterminate?: boolean
  onCheckedChange?: (checked: boolean) => void
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, indeterminate, onChange, onCheckedChange, checked, disabled, ...props }, ref) => {
    const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
      onChange?.(event)
      onCheckedChange?.(event.target.checked)
    }

    const handleClick = () => {
      if (disabled) return
      onCheckedChange?.(!checked)
    }

    return (
      <div className="relative inline-flex cursor-pointer">
        <input
          ref={ref}
          type="checkbox"
          className="peer sr-only"
          checked={checked}
          onChange={handleChange}
          disabled={disabled}
          {...props}
        />
        <div
          onClick={handleClick}
          className={cn(
            'flex h-4 w-4 items-center justify-center rounded border border-input bg-background',
            'peer-focus-visible:outline-none peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2',
            'peer-disabled:cursor-not-allowed peer-disabled:opacity-50',
            'peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground',
            disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:border-primary/50',
            className
          )}
        >
          {indeterminate ? <Minus className="h-3 w-3" /> : checked && <Check className="h-3 w-3" />}
        </div>
      </div>
    )
  }
)

Checkbox.displayName = 'Checkbox'
