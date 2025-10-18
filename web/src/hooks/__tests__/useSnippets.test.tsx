import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import * as snippetsApi from '@/lib/api/snippets'
import { useExecuteSnippet, useSnippets } from '@/hooks/useSnippets'

describe('useSnippets hooks', () => {
  it('fetches snippets with provided filters', async () => {
    const queryClient = new QueryClient()
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const apiSpy = vi.spyOn(snippetsApi, 'fetchSnippets').mockResolvedValue([
      {
        id: 'snp-1',
        name: 'List processes',
        scope: 'global',
        command: 'ps aux',
      },
    ])

    const { result } = renderHook(() => useSnippets({ scope: 'global', connectionId: 'conn-1' }), {
      wrapper,
    })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(apiSpy).toHaveBeenCalledWith({ scope: 'global', connectionId: 'conn-1' })
    expect(result.current.data).toHaveLength(1)
  })

  it('executes snippet mutation', async () => {
    const queryClient = new QueryClient()
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const execSpy = vi.spyOn(snippetsApi, 'executeSnippet').mockResolvedValue(undefined)

    const { result } = renderHook(() => useExecuteSnippet(), { wrapper })

    result.current.mutate({ sessionId: 'sess-1', snippetId: 'snp-1' })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(execSpy).toHaveBeenCalledWith('sess-1', 'snp-1')
  })
})
