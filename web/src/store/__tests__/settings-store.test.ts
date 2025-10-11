import { describe, beforeEach, it, expect } from 'vitest'
import { useSettingsStore, getDefaultPreferences } from '@/store/settings-store'

describe('settings store', () => {
  beforeEach(() => {
    useSettingsStore.setState({ preferences: getDefaultPreferences() })
  })

  it('initialises with default preferences', () => {
    const state = useSettingsStore.getState()
    expect(state.preferences.appearance.theme).toBe('system')
    expect(state.preferences.terminal.fontSize).toBeGreaterThan(0)
  })

  it('updates appearance theme and density', () => {
    const { updateAppearance } = useSettingsStore.getState()
    updateAppearance({ theme: 'dark', density: 'compact' })

    const state = useSettingsStore.getState()
    expect(state.preferences.appearance.theme).toBe('dark')
    expect(state.preferences.appearance.density).toBe('compact')
  })

  it('updates notifications', () => {
    const { updateNotifications } = useSettingsStore.getState()
    updateNotifications({ emailAlerts: false, weeklySummary: false })

    const state = useSettingsStore.getState()
    expect(state.preferences.notifications.emailAlerts).toBe(false)
    expect(state.preferences.notifications.weeklySummary).toBe(false)
  })

  it('resets to defaults', () => {
    const { updateSessions, resetPreferences } = useSettingsStore.getState()
    updateSessions({ idleTimeoutMinutes: 15 })
    resetPreferences()

    const state = useSettingsStore.getState()
    expect(state.preferences.sessions.idleTimeoutMinutes).toBe(
      getDefaultPreferences().sessions.idleTimeoutMinutes
    )
  })
})
