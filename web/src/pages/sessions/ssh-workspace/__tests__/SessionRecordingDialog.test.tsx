import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { SessionRecordingDialog } from '../SessionRecordingDialog'
import type { SessionRecordingStatus } from '@/types/session-recording'
import { downloadSessionRecording } from '@/lib/api/session-recordings'

const createMock = vi.fn()
const disposeMock = vi.fn()
vi.mock('asciinema-player', () => ({
  create: (...args: unknown[]) => {
    createMock(...args)
    return { dispose: disposeMock }
  },
}))

const ungzipMock = vi.fn(() => 'fake cast')
vi.mock('pako', () => ({
  ungzip: (...args: unknown[]) => ungzipMock(...args),
}))

vi.mock('@/lib/api/session-recordings', () => ({
  downloadSessionRecording: vi.fn(),
}))

const mockedDownload = vi.mocked(downloadSessionRecording)

describe('SessionRecordingDialog', () => {
  const createObjectURLMock = vi.fn(() => 'blob:mock')
  const revokeObjectURLMock = vi.fn()

  beforeAll(() => {
    Object.defineProperty(global.URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURLMock,
    })
    Object.defineProperty(global.URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURLMock,
    })
  })

  beforeEach(() => {
    mockedDownload.mockReset()
    createMock.mockClear()
    disposeMock.mockClear()
    ungzipMock.mockClear()
    createObjectURLMock.mockClear()
    revokeObjectURLMock.mockClear()
  })

  it('displays recorded session details and allows downloading the artifact', async () => {
    const buffer = new TextEncoder().encode('payload').buffer
    const blob = {
      arrayBuffer: vi.fn(() => Promise.resolve(buffer)),
    } as unknown as Blob
    mockedDownload.mockResolvedValue(blob)

    const status: SessionRecordingStatus = {
      session_id: 'sess-1',
      active: false,
      started_at: new Date('2024-05-01T10:00:00Z').toISOString(),
      last_event_at: new Date('2024-05-01T10:05:00Z').toISOString(),
      bytes_recorded: 2048,
      recording_mode: 'forced',
      record: {
        record_id: 'rec-1',
        session_id: 'sess-1',
        storage_path: 'records/sess-1.cast.gz',
        storage_kind: 'filesystem',
        size_bytes: 2048,
        duration_seconds: 120,
        checksum: 'abc123',
        created_at: new Date('2024-05-01T10:05:00Z').toISOString(),
        retention_until: new Date('2024-06-01T10:05:00Z').toISOString(),
      },
    }

    render(
      <SessionRecordingDialog
        open
        onClose={() => void 0}
        sessionId={status.session_id}
        status={status}
        isLoading={false}
        onRefresh={vi.fn()}
      />
    )

    await waitFor(() => expect(mockedDownload).toHaveBeenCalledWith('rec-1'))
    await waitFor(() => expect(screen.getByText(/Duration: 2m 0s/i)).toBeInTheDocument())

    const downloadButton = screen.getByRole('button', { name: /download/i })
    await userEvent.click(downloadButton)
    expect(createObjectURLMock).toHaveBeenCalledWith(blob)
  })

  it('invokes refresh handler when requested', async () => {
    const onRefresh = vi.fn()
    mockedDownload.mockResolvedValue(new Blob(['payload']))

    const status: SessionRecordingStatus = {
      session_id: 'sess-2',
      active: false,
      started_at: undefined,
      last_event_at: undefined,
      bytes_recorded: 0,
      recording_mode: 'optional',
      record: {
        record_id: 'rec-2',
        session_id: 'sess-2',
        storage_path: 'path',
        storage_kind: 'filesystem',
        size_bytes: 0,
        duration_seconds: 0,
        checksum: undefined,
        created_at: undefined,
        retention_until: undefined,
      },
    }

    render(
      <SessionRecordingDialog
        open
        onClose={() => void 0}
        sessionId={status.session_id}
        status={status}
        isLoading={false}
        onRefresh={onRefresh}
      />
    )

    await waitFor(() => expect(mockedDownload).toHaveBeenCalled())
    const refreshButton = screen.getByRole('button', { name: /refresh/i })
    await userEvent.click(refreshButton)
    expect(onRefresh).toHaveBeenCalled()
  })

  it('renders active recording state without fetching artifacts', () => {
    const status: SessionRecordingStatus = {
      session_id: 'sess-3',
      active: true,
      started_at: new Date().toISOString(),
      last_event_at: new Date().toISOString(),
      bytes_recorded: 512,
      recording_mode: 'forced',
      record: null,
    }

    render(
      <SessionRecordingDialog
        open
        onClose={() => void 0}
        sessionId={status.session_id}
        status={status}
        isLoading={false}
      />
    )

    expect(screen.getByText(/Recording in progress/i)).toBeInTheDocument()
    expect(mockedDownload).not.toHaveBeenCalled()
  })

  it('shows inactive message when recording is not available', () => {
    const status: SessionRecordingStatus = {
      session_id: 'sess-4',
      active: false,
      started_at: undefined,
      last_event_at: undefined,
      bytes_recorded: 0,
      recording_mode: 'optional',
      record: null,
    }

    render(
      <SessionRecordingDialog
        open
        onClose={() => void 0}
        sessionId={status.session_id}
        status={status}
        isLoading={false}
      />
    )

    expect(screen.getByText(/Recording is not active for this session/i)).toBeInTheDocument()
  })
})
