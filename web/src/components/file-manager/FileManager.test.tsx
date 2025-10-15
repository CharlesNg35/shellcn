import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import type { ReactElement } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { webcrypto } from 'node:crypto'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { FileManager } from '@/components/file-manager/FileManager'
import type { SftpEntry } from '@/types/sftp'

const mockUseSftpDirectory = vi.fn()
const mockUseSftpUpload = vi.fn()
const mockUseSftpDeleteFile = vi.fn()
const mockUseSftpDeleteDirectory = vi.fn()

vi.mock('@/hooks/useSftp', () => ({
  useSftpDirectory: (...args: unknown[]) => mockUseSftpDirectory(...args),
  useSftpUpload: (...args: unknown[]) => mockUseSftpUpload(...args),
  useSftpDeleteFile: (...args: unknown[]) => mockUseSftpDeleteFile(...args),
  useSftpDeleteDirectory: (...args: unknown[]) => mockUseSftpDeleteDirectory(...args),
}))

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => ({
    permissions: [],
    hasPermission: () => true,
    hasAnyPermission: () => true,
    hasAllPermissions: () => true,
    isLoading: false,
    refetch: vi.fn(),
  }),
}))

vi.mock('@/lib/utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
    promise: vi.fn(),
    custom: vi.fn(),
  },
}))

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 0,
      },
    },
  })
}

function renderWithClient(ui: ReactElement) {
  const queryClient = createQueryClient()
  const view = render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
  return { queryClient, ...view }
}

const baseEntries: SftpEntry[] = [
  {
    name: 'logs',
    path: 'logs',
    type: 'directory',
    isDir: true,
    size: 0,
    mode: 'drwxr-xr-x',
    modifiedAt: new Date('2024-01-01T00:00:00Z'),
  },
  {
    name: 'config.yaml',
    path: 'config.yaml',
    type: 'file',
    isDir: false,
    size: 2048,
    mode: '-rw-r--r--',
    modifiedAt: new Date('2024-01-02T12:00:00Z'),
  },
]

beforeAll(() => {
  if (!globalThis.crypto) {
    Object.defineProperty(globalThis, 'crypto', {
      value: webcrypto,
      configurable: true,
    })
  }
})

describe('FileManager component', () => {
  const uploadMutateAsync = vi.fn()
  const deleteFileMutateAsync = vi.fn()
  const deleteDirectoryMutateAsync = vi.fn()

  beforeEach(() => {
    uploadMutateAsync.mockReset()
    deleteFileMutateAsync.mockReset()
    deleteDirectoryMutateAsync.mockReset()
    mockUseSftpDirectory.mockReset()
    mockUseSftpUpload.mockReset()
    mockUseSftpDeleteFile.mockReset()
    mockUseSftpDeleteDirectory.mockReset()

    mockUseSftpDirectory.mockReturnValue({
      data: { path: '.', entries: baseEntries },
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })

    mockUseSftpUpload.mockReturnValue({
      mutateAsync: uploadMutateAsync,
      mutate: vi.fn(),
      isPending: false,
      reset: vi.fn(),
      status: 'idle',
    })

    mockUseSftpDeleteFile.mockReturnValue({
      mutateAsync: deleteFileMutateAsync,
      mutate: vi.fn(),
      isPending: false,
      status: 'idle',
      reset: vi.fn(),
    })

    mockUseSftpDeleteDirectory.mockReturnValue({
      mutateAsync: deleteDirectoryMutateAsync,
      mutate: vi.fn(),
      isPending: false,
      status: 'idle',
      reset: vi.fn(),
    })
  })

  it('renders directory entries', () => {
    renderWithClient(<FileManager sessionId="sess-1" />)

    expect(screen.getByText('logs')).toBeInTheDocument()
    expect(screen.getByText('config.yaml')).toBeInTheDocument()
  })

  it('invokes delete mutation when delete button is clicked', async () => {
    deleteDirectoryMutateAsync.mockResolvedValueOnce(undefined)
    deleteFileMutateAsync.mockResolvedValueOnce(undefined)

    renderWithClient(<FileManager sessionId="sess-1" />)

    const deleteButtons = screen.getAllByLabelText('Delete')

    fireEvent.click(deleteButtons[0])
    await waitFor(() => expect(deleteDirectoryMutateAsync).toHaveBeenCalled())

    fireEvent.click(deleteButtons[1])
    await waitFor(() => expect(deleteFileMutateAsync).toHaveBeenCalled())
  })

  it('uploads files when a selection is made', async () => {
    uploadMutateAsync.mockResolvedValueOnce({ path: 'upload.txt', uploadedBytes: 6, nextOffset: 6 })
    renderWithClient(<FileManager sessionId="sess-1" />)

    const fileInput = screen.getByTestId('sftp-upload-input') as HTMLInputElement
    const file = new File(['upload'], 'upload.txt', { type: 'text/plain' })

    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => expect(uploadMutateAsync).toHaveBeenCalled())
    expect(uploadMutateAsync).toHaveBeenCalledWith({
      path: 'upload.txt',
      blob: file,
      options: {
        createParents: true,
        onChunk: expect.any(Function),
      },
    })
  })
})
