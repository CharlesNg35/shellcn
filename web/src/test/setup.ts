import '@testing-library/jest-dom/vitest'
import { afterEach, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import { createElement } from 'react'
import type { ChangeEvent } from 'react'

// Polyfill ResizeObserver for Radix UI components
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

vi.mock('@monaco-editor/react', () => ({
  __esModule: true,
  default: ({
    value,
    onChange,
  }: {
    value?: string
    onChange?: (value: string | undefined) => void
  }) =>
    createElement('textarea', {
      'data-testid': 'mock-monaco-editor',
      value: value ?? '',
      onChange: (event: ChangeEvent<HTMLTextAreaElement>) => onChange?.(event.target.value),
    }),
}))

afterEach(() => {
  cleanup()
})
