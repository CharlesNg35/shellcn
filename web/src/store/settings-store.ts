import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemePreference = 'light' | 'dark' | 'system'
export type CursorStyle = 'block' | 'underline' | 'bar'
export type InterfaceDensity = 'comfortable' | 'cozy' | 'compact'

export interface TerminalPreferences {
  fontFamily: string
  fontSize: number
  cursorStyle: CursorStyle
  cursorBlink: boolean
  theme: 'dark' | 'light' | 'solarized'
}

export interface AppearancePreferences {
  theme: ThemePreference
  language: string
  accentColor: 'blue' | 'emerald' | 'violet' | 'amber'
  reduceMotion: boolean
  density: InterfaceDensity
}

export interface NotificationPreferences {
  emailAlerts: boolean
  desktopAlerts: boolean
  securityAlerts: boolean
  productUpdates: boolean
  weeklySummary: boolean
}

export interface SessionPreferences {
  idleTimeoutMinutes: number
  warnBeforeTimeoutMinutes: number
  rememberDevices: boolean
  autoReconnectSessions: boolean
}

export interface UiPreferences {
  sidebarCollapsed: boolean
  showConnectionHints: boolean
}

export interface UserPreferences {
  appearance: AppearancePreferences
  terminal: TerminalPreferences
  notifications: NotificationPreferences
  sessions: SessionPreferences
  ui: UiPreferences
}

const defaultPreferences: UserPreferences = {
  appearance: {
    theme: 'system',
    language: 'en',
    accentColor: 'blue',
    reduceMotion: false,
    density: 'comfortable',
  },
  terminal: {
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    fontSize: 14,
    cursorStyle: 'block',
    cursorBlink: true,
    theme: 'dark',
  },
  notifications: {
    emailAlerts: true,
    desktopAlerts: true,
    securityAlerts: true,
    productUpdates: false,
    weeklySummary: true,
  },
  sessions: {
    idleTimeoutMinutes: 60,
    warnBeforeTimeoutMinutes: 5,
    rememberDevices: true,
    autoReconnectSessions: true,
  },
  ui: {
    sidebarCollapsed: false,
    showConnectionHints: true,
  },
}

function mergePreferences(
  base: UserPreferences,
  next: Partial<UserPreferences> | undefined
): UserPreferences {
  if (!next) {
    return base
  }

  return {
    appearance: { ...base.appearance, ...(next.appearance ?? {}) },
    terminal: { ...base.terminal, ...(next.terminal ?? {}) },
    notifications: { ...base.notifications, ...(next.notifications ?? {}) },
    sessions: { ...base.sessions, ...(next.sessions ?? {}) },
    ui: { ...base.ui, ...(next.ui ?? {}) },
  }
}

interface SettingsState {
  preferences: UserPreferences
  updateAppearance: (prefs: Partial<AppearancePreferences>) => void
  updateTerminal: (prefs: Partial<TerminalPreferences>) => void
  updateNotifications: (prefs: Partial<NotificationPreferences>) => void
  updateSessions: (prefs: Partial<SessionPreferences>) => void
  updateUi: (prefs: Partial<UiPreferences>) => void
  setPreferences: (prefs: Partial<UserPreferences>) => void
  resetPreferences: () => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      preferences: mergePreferences(defaultPreferences, undefined),
      updateAppearance: (prefs) =>
        set((state) => ({
          preferences: {
            ...state.preferences,
            appearance: { ...state.preferences.appearance, ...prefs },
          },
        })),
      updateTerminal: (prefs) =>
        set((state) => ({
          preferences: {
            ...state.preferences,
            terminal: { ...state.preferences.terminal, ...prefs },
          },
        })),
      updateNotifications: (prefs) =>
        set((state) => ({
          preferences: {
            ...state.preferences,
            notifications: { ...state.preferences.notifications, ...prefs },
          },
        })),
      updateSessions: (prefs) =>
        set((state) => ({
          preferences: {
            ...state.preferences,
            sessions: { ...state.preferences.sessions, ...prefs },
          },
        })),
      updateUi: (prefs) =>
        set((state) => ({
          preferences: {
            ...state.preferences,
            ui: { ...state.preferences.ui, ...prefs },
          },
        })),
      setPreferences: (prefs) =>
        set((state) => ({
          preferences: mergePreferences(state.preferences, prefs),
        })),
      resetPreferences: () =>
        set({
          preferences: mergePreferences(defaultPreferences, undefined),
        }),
    }),
    {
      name: 'shellcn-settings',
      partialize: (state) => ({ preferences: state.preferences }),
      version: 1,
    }
  )
)

export function getDefaultPreferences(): UserPreferences {
  return mergePreferences(defaultPreferences, undefined)
}
