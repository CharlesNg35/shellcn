import { useMemo } from 'react'
import { cn } from '@/lib/utils/cn'

interface PasswordStrengthMeterProps {
  password: string
}

const strengthChecks = [
  {
    label: 'At least 8 characters',
    check: (password: string) => password.length >= 8,
  },
  {
    label: 'Includes uppercase letter',
    check: (password: string) => /[A-Z]/.test(password),
  },
  {
    label: 'Includes number',
    check: (password: string) => /[0-9]/.test(password),
  },
  {
    label: 'Includes symbol',
    check: (password: string) => /[^A-Za-z0-9]/.test(password),
  },
]

export function PasswordStrengthMeter({ password }: PasswordStrengthMeterProps) {
  const { score, checks } = useMemo(() => {
    const results = strengthChecks.map((item) => ({
      label: item.label,
      passed: item.check(password),
    }))

    const count = results.filter((item) => item.passed).length

    return {
      score: count,
      checks: results,
    }
  }, [password])

  const strengthLabel = score <= 1 ? 'Weak' : score === 2 ? 'Fair' : score === 3 ? 'Good' : 'Strong'

  return (
    <div className="mt-3 space-y-3">
      <div className="flex h-2 overflow-hidden rounded-full bg-muted">
        {strengthChecks.map((_item, index) => (
          <div
            key={_item.label}
            className={cn(
              'transition-all duration-300 ease-in-out',
              index < score ? 'bg-primary' : 'bg-muted-foreground/20'
            )}
            style={{ width: '25%' }}
          />
        ))}
      </div>
      <div className="flex items-center justify-between text-xs font-medium text-muted-foreground">
        <span>Password strength</span>
        <span
          className={cn(
            'uppercase tracking-wide',
            score <= 1
              ? 'text-destructive'
              : score === 2
                ? 'text-amber-600'
                : score === 3
                  ? 'text-primary'
                  : 'text-emerald-600'
          )}
        >
          {strengthLabel}
        </span>
      </div>
      <ul className="space-y-1 text-xs text-muted-foreground">
        {checks.map((item) => (
          <li
            key={item.label}
            className={cn(
              'flex items-center gap-2',
              item.passed ? 'text-emerald-600' : 'text-muted-foreground'
            )}
          >
            <span
              className={cn(
                'inline-flex h-1.5 w-1.5 rounded-full',
                item.passed ? 'bg-emerald-500' : 'bg-muted-foreground/40'
              )}
            />
            {item.label}
          </li>
        ))}
      </ul>
    </div>
  )
}
