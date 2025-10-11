import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { useSettings } from '@/hooks/useSettings'
import type { ThemePreference } from '@/store/settings-store'

interface AppearanceSettingsPanelProps {
  className?: string
}

const THEME_OPTIONS: Array<{ value: ThemePreference; label: string; description: string }> = [
  {
    value: 'system',
    label: 'Match system',
    description: 'Follow your operating system preference.',
  },
  { value: 'light', label: 'Light', description: 'Bright palette with elevated surfaces.' },
  { value: 'dark', label: 'Dark', description: 'Low-light friendly high contrast palette.' },
]

export function AppearanceSettingsPanel({ className }: AppearanceSettingsPanelProps) {
  const { theme, setTheme } = useSettings()

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Theme</CardTitle>
          <CardDescription>Select how ShellCN should render UI colors.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-3">
          {THEME_OPTIONS.map((option) => {
            const isActive = theme === option.value
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => setTheme(option.value)}
                className={cn(
                  'flex flex-col rounded-lg border border-border/60 bg-background px-4 py-3 text-left transition hover:border-border',
                  isActive && 'border-primary shadow-sm'
                )}
                aria-pressed={isActive}
              >
                <span className="text-sm font-semibold text-foreground">{option.label}</span>
                <span className="mt-1 text-xs text-muted-foreground">{option.description}</span>
              </button>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}
