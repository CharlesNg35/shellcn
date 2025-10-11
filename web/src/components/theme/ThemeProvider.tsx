import { useCallback, useEffect, useMemo } from 'react'
import { useSettings } from '@/hooks/useSettings'
import type { Theme } from './theme-context'
import { ThemeProviderContext } from './theme-context'

const THEME_CLASSNAMES: Theme[] = ['light', 'dark']

interface ThemeProviderProps {
  children: React.ReactNode
}

export function ThemeProvider({ children }: ThemeProviderProps) {
  const { theme, setTheme } = useSettings()

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    const root = window.document.documentElement

    const applyTheme = (value: Theme) => {
      THEME_CLASSNAMES.forEach((name) => root.classList.remove(name))

      if (value === 'system') {
        const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
        root.classList.add(systemPrefersDark ? 'dark' : 'light')
        return
      }

      root.classList.add(value)
    }

    applyTheme(theme)

    if (theme !== 'system') {
      return
    }

    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const listener = () => applyTheme('system')
    media.addEventListener('change', listener)
    return () => media.removeEventListener('change', listener)
  }, [theme])

  const updateTheme = useCallback(
    (value: Theme) => {
      setTheme(value)
    },
    [setTheme]
  )

  const value = useMemo(
    () => ({
      theme,
      setTheme: updateTheme,
    }),
    [theme, updateTheme]
  )

  return <ThemeProviderContext.Provider value={value}>{children}</ThemeProviderContext.Provider>
}
