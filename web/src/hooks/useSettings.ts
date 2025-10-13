import { useShallow } from 'zustand/react/shallow'
import { useSettingsStore, type ThemePreference } from '@/store/settings-store'

interface SettingsSelector {
  theme: ThemePreference
  setTheme: (theme: ThemePreference) => void
}

const selector = (state: SettingsSelector) => ({
  theme: state.theme,
  setTheme: state.setTheme,
})

export function useSettings() {
  return useSettingsStore(useShallow(selector))
}
