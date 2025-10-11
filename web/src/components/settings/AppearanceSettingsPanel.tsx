import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Checkbox } from '@/components/ui/Checkbox'
import { useSettingsPreferences } from '@/hooks/useSettingsPreferences'

const themeOptions = [
  { label: 'System', value: 'system' as const },
  { label: 'Light', value: 'light' as const },
  { label: 'Dark', value: 'dark' as const },
]

const accentOptions: Array<{ label: string; value: 'blue' | 'emerald' | 'violet' | 'amber' }> = [
  { label: 'Blue', value: 'blue' },
  { label: 'Emerald', value: 'emerald' },
  { label: 'Violet', value: 'violet' },
  { label: 'Amber', value: 'amber' },
]

const densityOptions = [
  { label: 'Comfortable', value: 'comfortable' as const },
  { label: 'Cozy', value: 'cozy' as const },
  { label: 'Compact', value: 'compact' as const },
]

const languageOptions = [
  { label: 'English', value: 'en' },
  { label: 'Français', value: 'fr' },
  { label: 'Deutsch', value: 'de' },
]

interface AppearanceSettingsPanelProps {
  className?: string
}

export function AppearanceSettingsPanel({ className }: AppearanceSettingsPanelProps) {
  const { preferences, updateAppearance } = useSettingsPreferences()
  const appearance = preferences.appearance

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Theme</CardTitle>
          <CardDescription>Choose light, dark, or system-based theming.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {themeOptions.map((option) => (
              <Button
                key={option.value}
                type="button"
                variant={appearance.theme === option.value ? 'secondary' : 'outline'}
                onClick={() => updateAppearance({ theme: option.value })}
              >
                {option.label}
              </Button>
            ))}
          </div>
          <div>
            <p className="text-sm font-semibold text-foreground">Reduce motion</p>
            <label className="mt-2 flex items-center gap-3 text-sm text-muted-foreground">
              <Checkbox
                checked={appearance.reduceMotion}
                onCheckedChange={(checked) => updateAppearance({ reduceMotion: Boolean(checked) })}
              />
              Prefer reduced motion and animations
            </label>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Accent color</CardTitle>
          <CardDescription>Personalize key highlights across the interface.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {accentOptions.map((option) => (
            <Button
              key={option.value}
              type="button"
              variant={appearance.accentColor === option.value ? 'secondary' : 'outline'}
              className={cn('px-3', appearance.accentColor === option.value && 'shadow-inner')}
              onClick={() => updateAppearance({ accentColor: option.value })}
            >
              {option.label}
            </Button>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Interface density</CardTitle>
          <CardDescription>Control spacing for tables, lists, and navigation.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {densityOptions.map((option) => (
            <Button
              key={option.value}
              type="button"
              variant={appearance.density === option.value ? 'secondary' : 'outline'}
              onClick={() => updateAppearance({ density: option.value })}
            >
              {option.label}
            </Button>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Language</CardTitle>
          <CardDescription>Select your preferred interface language.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {languageOptions.map((option) => (
            <Button
              key={option.value}
              type="button"
              variant={appearance.language === option.value ? 'secondary' : 'outline'}
              onClick={() => updateAppearance({ language: option.value })}
            >
              {option.label}
            </Button>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}
