import { act, render, screen } from '@testing-library/react'
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import SshTerminal from '@/components/workspace/SshTerminal'
import type { SshTerminalEvent } from '@/types/ssh'

const writeMock = vi.fn()
const fitMock = vi.fn()
const disposeMock = vi.fn()
const loadAddonMock = vi.fn()
const searchFindNextMock = vi.fn(() => true)
const searchFindPreviousMock = vi.fn(() => true)

const { TerminalStub, FitAddonStub, WebglAddonStub, SearchAddonStub } = vi.hoisted(() => {
  class TerminalStub {
    public loadAddon = loadAddonMock
    public write = writeMock
    public dispose = disposeMock
    public focus = vi.fn()
    public open = vi.fn()
    public clear = vi.fn()
    public refresh = vi.fn()
    public cols = 80
    public rows = 24
    public options: {
      fontSize: number
      fontFamily?: string
      scrollback?: number
      theme?: Record<string, unknown>
    } = { fontSize: 14 }
    public onData = vi.fn(() => ({ dispose: vi.fn() }))
    public onResize = vi.fn(() => ({ dispose: vi.fn() }))
    constructor(public readonly config: Record<string, unknown>) {
      if (typeof config.fontSize === 'number') {
        this.options.fontSize = config.fontSize
      }
      if (typeof config.fontFamily === 'string') {
        this.options.fontFamily = config.fontFamily
      }
      if (typeof config.scrollback === 'number') {
        this.options.scrollback = config.scrollback
      }
      if (config.theme) {
        this.options.theme = config.theme as Record<string, unknown>
      }
    }
  }

  class FitAddonStub {
    public fit = fitMock
    public dispose = disposeMock
  }

  class WebglAddonStub {
    public dispose = disposeMock
  }

  class SearchAddonStub {
    public findNext = searchFindNextMock
    public findPrevious = searchFindPreviousMock
    public dispose = disposeMock
  }

  return { TerminalStub, FitAddonStub, WebglAddonStub, SearchAddonStub }
})

vi.mock('@xterm/xterm', () => ({
  Terminal: TerminalStub,
}))

vi.mock('@xterm/addon-fit', () => ({
  FitAddon: FitAddonStub,
}))

vi.mock('@xterm/addon-webgl', () => ({
  WebglAddon: WebglAddonStub,
}))

vi.mock('@xterm/addon-search', () => ({
  SearchAddon: SearchAddonStub,
}))

const streamResult = {
  isConnected: true,
  send: vi.fn(),
  close: vi.fn(),
  lastMessage: null,
}

let capturedHandler: ((event: SshTerminalEvent) => void) | null = null

vi.mock('@/hooks/useSshTerminalStream', () => ({
  useSshTerminalStream: (options: { onEvent?: (event: SshTerminalEvent) => void }) => {
    capturedHandler = options.onEvent ?? null
    return streamResult
  },
}))

class ResizeObserverStub {
  observe() {}
  disconnect() {}
}

beforeAll(() => {
  // @ts-expect-error assign test stub
  global.ResizeObserver = ResizeObserverStub
})

afterAll(() => {
  // @ts-expect-error cleanup stub
  delete global.ResizeObserver
})

describe('SshTerminal component', () => {
  let originalRaf: typeof window.requestAnimationFrame
  let originalCancelRaf: typeof window.cancelAnimationFrame
  let originalIdle: typeof window.requestIdleCallback
  let originalCancelIdle: typeof window.cancelIdleCallback

  beforeEach(() => {
    originalRaf = window.requestAnimationFrame
    originalCancelRaf = window.cancelAnimationFrame
    originalIdle = window.requestIdleCallback
    originalCancelIdle = window.cancelIdleCallback

    window.requestAnimationFrame = (callback) => window.setTimeout(() => callback(16), 0)
    window.cancelAnimationFrame = (handle) => window.clearTimeout(handle)
    window.requestIdleCallback = (callback) =>
      window.setTimeout(
        () =>
          callback({
            didTimeout: false,
            timeRemaining: () => 50,
          }),
        0
      )
    window.cancelIdleCallback = (handle) => window.clearTimeout(handle)

    vi.useFakeTimers()
    writeMock.mockClear()
    fitMock.mockClear()
    disposeMock.mockClear()
    loadAddonMock.mockClear()
    streamResult.send.mockClear()
    streamResult.close.mockClear()
    capturedHandler = null
    searchFindNextMock.mockClear()
    searchFindPreviousMock.mockClear()
  })

  afterEach(() => {
    window.requestAnimationFrame = originalRaf
    window.cancelAnimationFrame = originalCancelRaf
    window.requestIdleCallback = originalIdle
    window.cancelIdleCallback = originalCancelIdle
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('renders terminal container and writes streamed data', async () => {
    render(<SshTerminal sessionId="sess-1" />)

    await act(async () => {
      vi.runAllTimers()
    })
    expect(fitMock).toHaveBeenCalled()

    expect(screen.getByTestId('ssh-terminal-canvas')).toBeInTheDocument()
    expect(writeMock).not.toHaveBeenCalled()

    act(() => {
      capturedHandler?.({
        stream: 'ssh.terminal',
        event: 'stdout',
        sessionId: 'sess-1',
        text: 'hello world',
      })
      vi.runAllTimers()
    })

    expect(writeMock).toHaveBeenCalledWith('hello world')
  })

  it('updates status when session closes', async () => {
    render(<SshTerminal sessionId="sess-2" />)
    await act(async () => {
      vi.runAllTimers()
    })
    expect(fitMock).toHaveBeenCalled()

    act(() => {
      capturedHandler?.({
        stream: 'ssh.terminal',
        event: 'closed',
        sessionId: 'sess-2',
        message: 'Session finished',
      })
    })

    expect(screen.getByText('Session finished')).toBeInTheDocument()
  })
})
