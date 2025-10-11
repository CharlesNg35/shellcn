import { describe, beforeEach, it, expect } from 'vitest'
import { useSettingsStore, type ThemePreference } from '@/store/settings-store'

describe('settings store', () => {
  beforeEach(() => {
    useSettingsStore.setState({ theme: 'system' })
  })

  it('defaults to system theme', () => {
    const state = useSettingsStore.getState()
    expect(state.theme).toBe<'system'>('system')
  })

  it('updates stored theme', () => {
    const { setTheme } = useSettingsStore.getState()
    setTheme('dark')

    const state = useSettingsStore.getState()
    expect(state.theme).toBe<ThemePreference>('dark')
  })
})
