import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { mapTerminalMessage, useSshTerminalStream } from '@/hooks/useSshTerminalStream'
import type { RealtimeMessage } from '@/types/realtime'

const mockUseWebSocket = vi.fn(() => ({
  isConnected: true,
  send: vi.fn(),
  close: vi.fn(),
  lastMessage: null,
  url: '/ws?token=token-123',
  ready: true,
}))

vi.mock('@/hooks/useAuthenticatedWebSocket', () => ({
  useAuthenticatedWebSocket: (...args: unknown[]) => mockUseWebSocket(...args),
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    tokens: { accessToken: 'token-123' },
    isAuthenticated: true,
  }),
}))

describe('useSshTerminalStream', () => {
  it('maps terminal messages and decodes base64 payloads', () => {
    const message: RealtimeMessage<{
      session_id: string
      connection_id: string
      payload: string
      encoding: string
      channel: string
    }> = {
      stream: 'ssh.terminal',
      event: 'stdout',
      data: {
        session_id: 'sess-1',
        connection_id: 'conn-1',
        payload: Buffer.from('hello').toString('base64'),
        encoding: 'base64',
        channel: 'stdout',
      },
      meta: undefined,
    }

    const mapped = mapTerminalMessage(message, 'sess-1')
    expect(mapped).toMatchObject({
      event: 'stdout',
      sessionId: 'sess-1',
      connectionId: 'conn-1',
      text: 'hello',
      channel: 'stdout',
    })
    expect(mapped?.raw).toBeInstanceOf(Uint8Array)
  })

  it('subscribes to websocket stream and invokes callback', () => {
    const handler = vi.fn()
    renderHook(() => useSshTerminalStream({ sessionId: 'sess-2', enabled: true, onEvent: handler }))

    const call = mockUseWebSocket.mock.calls[0] ?? []
    const options = (call.length === 1 ? call[0] : call[1]) as Record<string, unknown> | undefined
    expect(options?.enabled).toBe(true)

    const rawMessage: RealtimeMessage<{
      session_id: string
      payload: string
      encoding: string
    }> = {
      stream: 'ssh.terminal',
      event: 'stderr',
      data: {
        session_id: 'sess-2',
        payload: Buffer.from('error').toString('base64'),
        encoding: 'base64',
      },
      meta: undefined,
    }

    options?.onMessage?.(rawMessage)
    expect(handler).toHaveBeenCalledTimes(1)
    expect(handler.mock.calls[0]?.[0]?.text).toBe('error')
  })
})
