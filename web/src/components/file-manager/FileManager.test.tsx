import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import type { ReactElement } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { webcrypto } from 'node:crypto'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { FileManager } from '@/components/file-manager/FileManager'
import type { SftpEntry, SftpTransferRealtimeEvent } from '@/types/sftp'
import { resetSshWorkspaceStore, workspaceStoreMocks } from '@/store/ssh-workspace-store'

const mockUseSftpDirectory = vi.fn()
const mockUseSftpUpload = vi.fn()
const mockUseSftpDeleteFile = vi.fn()
const mockUseSftpDeleteDirectory = vi.fn()
const mockUseSftpTransfersStream = vi.fn()

const storeState = {
  sessions: {
    'sess-1': {
      sessionId: 'sess-1',
      browserPath: '.',
      showHidden: false,
      tabs: [],
      activeTabId: '',
      transfers: {},
      transferOrder: [],
    },
  },
}

let realtimeHandler: ((event: unknown) => void) | undefined

vi.mock('@/hooks/useSftp', () => ({
  useSftpDirectory: (...args: unknown[]) => mockUseSftpDirectory(...args),
  useSftpUpload: (...args: unknown[]) => mockUseSftpUpload(...args),
  useSftpDeleteFile: (...args: unknown[]) => mockUseSftpDeleteFile(...args),
  useSftpDeleteDirectory: (...args: unknown[]) => mockUseSftpDeleteDirectory(...args),
}))

vi.mock('@/hooks/useSftpTransfersStream', () => ({
  useSftpTransfersStream: (options: { onEvent?: (event: unknown) => void }) => {
    realtimeHandler = options?.onEvent
    return mockUseSftpTransfersStream(options)
  },
}))

vi.mock('@/store/ssh-workspace-store', () => {
  const ensureSessionMock = vi.fn()
  const setBrowserPathMock = vi.fn()
  const setShowHiddenMock = vi.fn()
  const upsertTransferMock = vi.fn()
  const updateTransferMock = vi.fn()
  const clearTransfersMock = vi.fn()
  const useStore = (selector: any) =>
    selector({
      sessions: storeState.sessions,
      ensureSession: ensureSessionMock,
      setBrowserPath: setBrowserPathMock,
      setShowHidden: setShowHiddenMock,
      upsertTransfer: upsertTransferMock,
      updateTransfer: updateTransferMock,
      clearCompletedTransfers: clearTransfersMock,
    })
  useStore.getState = () => ({ sessions: storeState.sessions })
  useStore.setState = (updater: any) => {
    if (typeof updater === 'function') {
      const result = updater({ sessions: storeState.sessions })
      if (result?.sessions) {
        storeState.sessions = result.sessions as typeof storeState.sessions
      }
    } else if (updater?.sessions) {
      storeState.sessions = updater.sessions as typeof storeState.sessions
    }
  }
  const reset = () => {
    storeState.sessions = {
      'sess-1': {
        sessionId: 'sess-1',
        browserPath: '.',
        showHidden: false,
        tabs: [],
        activeTabId: '',
        transfers: {},
        transferOrder: [],
      },
    }
    ensureSessionMock.mockReset()
    setBrowserPathMock.mockReset()
    setShowHiddenMock.mockReset()
    upsertTransferMock.mockReset()
    updateTransferMock.mockReset()
    clearTransfersMock.mockReset()
  }
  return {
    useSshWorkspaceStore: useStore,
    resetSshWorkspaceStore: reset,
    workspaceStoreMocks: {
      ensureSession: ensureSessionMock,
      setBrowserPath: setBrowserPathMock,
      setShowHidden: setShowHiddenMock,
      upsertTransfer: upsertTransferMock,
      updateTransfer: updateTransferMock,
      clearCompletedTransfers: clearTransfersMock,
    },
  }
})

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
    mockUseSftpTransfersStream.mockReset()
    realtimeHandler = undefined
    resetSshWorkspaceStore()

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

    mockUseSftpTransfersStream.mockReturnValue({
      isConnected: true,
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
    expect(workspaceStoreMocks.upsertTransfer).toHaveBeenCalled()
  })

  it('updates transfers when realtime events arrive', async () => {
    renderWithClient(<FileManager sessionId="sess-1" />)

    expect(typeof realtimeHandler).toBe('function')

    const event: SftpTransferRealtimeEvent = {
      event: 'sftp.transfer.started',
      status: 'started',
      payload: {
        sessionId: 'sess-1',
        connectionId: 'conn-42',
        userId: 'usr-2',
        path: 'remote/example.txt',
        direction: 'upload',
        transferId: 'transfer-1',
        status: 'started',
        bytesTransferred: 256,
        totalBytes: 1024,
        error: undefined,
      },
    }

    act(() => {
      realtimeHandler?.(event)
    })

    expect(workspaceStoreMocks.upsertTransfer).toHaveBeenCalled()
  })
})
