import { Moon, Sun } from 'lucide-react'
import { useTheme } from './useTheme'

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const isDark = theme === 'dark'

  const handleToggle = () => {
    setTheme(isDark ? 'light' : 'dark')
  }

  const label = isDark ? 'Switch to light theme' : 'Switch to dark theme'

  return (
    <button
      onClick={handleToggle}
      className="relative flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-background transition hover:bg-muted"
      aria-label={label}
      type="button"
    >
      <Sun
        className="h-4 w-4 rotate-0 scale-100 transition-transform duration-200 dark:-rotate-90 dark:scale-0"
        aria-hidden="true"
      />
      <Moon
        className="absolute h-4 w-4 rotate-90 scale-0 transition-transform duration-200 dark:rotate-0 dark:scale-100"
        aria-hidden="true"
      />
    </button>
  )
}
