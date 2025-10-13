import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemePreference = 'light' | 'dark' | 'system'

const DEFAULT_THEME: ThemePreference = 'system'

interface SettingsState {
  theme: ThemePreference
  setTheme: (theme: ThemePreference) => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      theme: DEFAULT_THEME,
      setTheme: (theme) => set({ theme }),
    }),
    {
      name: 'shellcn-settings',
      version: 1,
    }
  )
)
