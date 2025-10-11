import { useShallow } from 'zustand/react/shallow'
import {
  useSettingsStore,
  type AppearancePreferences,
  type NotificationPreferences,
  type SessionPreferences,
  type TerminalPreferences,
  type UiPreferences,
  type UserPreferences,
} from '@/store/settings-store'

interface SettingsSelector {
  preferences: UserPreferences
  updateAppearance: (prefs: Partial<AppearancePreferences>) => void
  updateTerminal: (prefs: Partial<TerminalPreferences>) => void
  updateNotifications: (prefs: Partial<NotificationPreferences>) => void
  updateSessions: (prefs: Partial<SessionPreferences>) => void
  updateUi: (prefs: Partial<UiPreferences>) => void
  setPreferences: (prefs: Partial<UserPreferences>) => void
  resetPreferences: () => void
}

const selector = (state: SettingsSelector) => ({
  preferences: state.preferences,
  updateAppearance: state.updateAppearance,
  updateTerminal: state.updateTerminal,
  updateNotifications: state.updateNotifications,
  updateSessions: state.updateSessions,
  updateUi: state.updateUi,
  setPreferences: state.setPreferences,
  resetPreferences: state.resetPreferences,
})

export function useSettingsPreferences() {
  return useSettingsStore(useShallow(selector))
}
