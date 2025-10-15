import { createElement, type ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getSftpListQueryKey,
  useSftpDeleteFile,
  useSftpDirectory,
  useSftpUpload,
} from '@/hooks/useSftp'

const mockList = vi.fn()
const mockUpload = vi.fn()
const mockDeleteFile = vi.fn()
const mockDeleteDirectory = vi.fn()

vi.mock('@/lib/api/sftp', () => ({
  sftpApi: {
    list: (...args: unknown[]) => mockList(...args),
    metadata: vi.fn(),
    readFile: vi.fn(),
    saveFile: vi.fn(),
    deleteFile: (...args: unknown[]) => mockDeleteFile(...args),
    deleteDirectory: (...args: unknown[]) => mockDeleteDirectory(...args),
    rename: vi.fn(),
    download: vi.fn(),
    upload: (...args: unknown[]) => mockUpload(...args),
  },
}))

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  })
}

function createWrapper(client: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client, children })
}

describe('useSftp hooks', () => {
  beforeEach(() => {
    mockList.mockReset()
    mockUpload.mockReset()
    mockDeleteFile.mockReset()
    mockDeleteDirectory.mockReset()
  })

  it('fetches directory entries', async () => {
    mockList.mockResolvedValueOnce({ path: '.', entries: [] })
    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)

    const { result } = renderHook(() => useSftpDirectory('sess-123'), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(mockList).toHaveBeenCalledWith('sess-123', undefined)
    queryClient.clear()
  })

  it('uploads files and invalidates directory cache', async () => {
    mockUpload.mockResolvedValueOnce({ path: 'logs/app.log', uploadedBytes: 3, nextOffset: 3 })

    const queryClient = createQueryClient()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = createWrapper(queryClient)

    const { result } = renderHook(() => useSftpUpload('sess-456'), { wrapper })

    await act(async () => {
      await result.current.mutateAsync({
        path: 'logs/app.log',
        blob: new Blob(['abc']),
      })
    })

    expect(mockUpload).toHaveBeenCalledTimes(1)
    expect(mockUpload).toHaveBeenCalledWith('sess-456', 'logs/app.log', expect.any(Blob), undefined)
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: ['sftp', 'list', 'sess-456'],
        exact: false,
      })
    )
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: getSftpListQueryKey('sess-456', 'logs'),
    })
    queryClient.clear()
  })

  it('deletes files via mutation', async () => {
    mockDeleteFile.mockResolvedValueOnce(undefined)
    const queryClient = createQueryClient()
    const wrapper = createWrapper(queryClient)

    const { result } = renderHook(() => useSftpDeleteFile('sess-789'), { wrapper })

    await act(async () => {
      await result.current.mutateAsync({ path: 'notes.txt' })
    })

    expect(mockDeleteFile).toHaveBeenCalledWith('sess-789', 'notes.txt')
    queryClient.clear()
  })
})
