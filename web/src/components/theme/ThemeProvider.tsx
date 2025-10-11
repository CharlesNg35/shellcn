import { useEffect, useState } from 'react'
import { useSettingsPreferences } from '@/hooks/useSettingsPreferences'
import type { Theme } from './theme-context'
import { ThemeProviderContext } from './theme-context'

interface ThemeProviderProps {
  children: React.ReactNode
  defaultTheme?: Theme
  storageKey?: string
}

export function ThemeProvider({
  children,
  defaultTheme = 'system',
  storageKey = 'shellcn-theme',
  ...props
}: ThemeProviderProps) {
  const { preferences, updateAppearance } = useSettingsPreferences()
  const preferredTheme = preferences.appearance.theme ?? defaultTheme
  const [theme, setTheme] = useState<Theme>(preferredTheme)

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    const stored = localStorage.getItem(storageKey) as Theme | null
    if (stored && stored !== preferredTheme) {
      updateAppearance({ theme: stored })
    } else {
      localStorage.setItem(storageKey, preferredTheme)
    }
    setTheme(preferredTheme)
  }, [preferredTheme, storageKey, updateAppearance])

  useEffect(() => {
    const root = window.document.documentElement

    root.classList.remove('light', 'dark')

    if (theme === 'system') {
      const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'

      root.classList.add(systemTheme)
      return
    }

    root.classList.add(theme)
  }, [theme])

  const value = {
    theme,
    setTheme: (theme: Theme) => {
      localStorage.setItem(storageKey, theme)
      setTheme(theme)
      updateAppearance({ theme })
    },
  }

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  )
}
