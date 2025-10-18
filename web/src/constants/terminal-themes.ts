/**
 * Terminal theme definitions
 */
import type { ITheme } from '@xterm/xterm'
import type { SSHThemeMode } from '@/types/protocol-settings'

export const DARK_THEME: ITheme = {
  background: '#1e1e2e',
  foreground: '#cdd6f4',
  cursor: '#f5e0dc',
  cursorAccent: '#1e1e2e',
  selectionBackground: '#45475a',
  selectionForeground: '#cdd6f4',
  black: '#45475a',
  red: '#f38ba8',
  green: '#a6e3a1',
  yellow: '#f9e2af',
  blue: '#89b4fa',
  magenta: '#f5c2e7',
  cyan: '#94e2d5',
  white: '#bac2de',
  brightBlack: '#585b70',
  brightRed: '#f38ba8',
  brightGreen: '#a6e3a1',
  brightYellow: '#f9e2af',
  brightBlue: '#89b4fa',
  brightMagenta: '#f5c2e7',
  brightCyan: '#94e2d5',
  brightWhite: '#a6adc8',
}

export const LIGHT_THEME: ITheme = {
  background: '#f8fafc',
  foreground: '#1e293b',
  cursor: '#0f172a',
  cursorAccent: '#f8fafc',
  selectionBackground: '#c7d2fe',
  selectionForeground: '#1e293b',
  black: '#334155',
  red: '#dc2626',
  green: '#15803d',
  yellow: '#b45309',
  blue: '#1d4ed8',
  magenta: '#7c3aed',
  cyan: '#0e7490',
  white: '#0f172a',
  brightBlack: '#475569',
  brightRed: '#ef4444',
  brightGreen: '#22c55e',
  brightYellow: '#f59e0b',
  brightBlue: '#3b82f6',
  brightMagenta: '#a855f7',
  brightCyan: '#06b6d4',
  brightWhite: '#1e293b',
}

/**
 * Resolves the current color scheme preference
 */
export function resolvePreferredScheme(): 'dark' | 'light' {
  if (typeof document !== 'undefined') {
    const root = document.documentElement
    if (root.classList.contains('dark')) {
      return 'dark'
    }
    if (root.classList.contains('light')) {
      return 'light'
    }
  }
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark'
    }
  }
  return 'light'
}

/**
 * Gets the appropriate theme based on mode
 */
export function resolveTheme(mode: SSHThemeMode): ITheme {
  switch (mode) {
    case 'force_light':
      return LIGHT_THEME
    case 'force_dark':
      return DARK_THEME
    default:
      return resolvePreferredScheme() === 'dark' ? DARK_THEME : LIGHT_THEME
  }
}
