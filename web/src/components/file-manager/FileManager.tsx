import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDropzone } from 'react-dropzone'
import { ArrowUp, FileText, Folder, Home, Loader2, RefreshCcw, Upload } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PERMISSIONS } from '@/constants/permissions'
import { cn } from '@/lib/utils/cn'
import { toast } from '@/lib/utils/toast'
import { toApiError } from '@/lib/api/http'
import {
  useSftpDeleteDirectory,
  useSftpDeleteFile,
  useSftpDirectory,
  useSftpUpload,
  getSftpListQueryKey,
} from '@/hooks/useSftp'
import { useSftpTransfersStream } from '@/hooks/useSftpTransfersStream'
import type { ActiveSessionParticipant } from '@/types/connections'
import type { SftpEntry, SftpTransferRealtimeEvent } from '@/types/sftp'
import {
  displayPath,
  extractNameFromPath,
  normalizePath,
  parentPath,
  resolveChildPath,
  resolveParticipantName,
  sortEntries,
} from './utils'
import type { TransferItem } from './types'
import { FileManagerToolbar } from './FileManagerToolbar'
import { FileManagerTable } from './FileManagerTable'
import { TransferSidebar } from './TransferSidebar'
import { useSshWorkspaceStore } from '@/store/ssh-workspace-store'
import { sftpApi } from '@/lib/api/sftp'

const EMPTY_TRANSFER_ORDER: string[] = []
const EMPTY_TRANSFERS_MAP: Record<string, TransferItem> = {}

interface FileManagerProps {
  sessionId: string
  initialPath?: string
  className?: string
  canWrite?: boolean
  currentUserId?: string
  currentUserName?: string
  participants?: Record<string, ActiveSessionParticipant>
  onOpenFile?: (entry: SftpEntry) => void
  showTransfers?: boolean
}

interface CreateTransferParams {
  name: string
  path: string
  direction: string
  size?: number
  userId?: string
  userName?: string
}

function createTransfer({
  name,
  path,
  direction,
  size,
  userId,
  userName,
}: CreateTransferParams): TransferItem {
  return {
    id: crypto.randomUUID ? crypto.randomUUID() : `${name}-${Date.now()}`,
    remoteId: undefined,
    name,
    path,
    direction,
    size: size ?? 0,
    uploaded: 0,
    status: 'pending',
    startedAt: new Date(),
    userId,
    userName,
  }
}

export function FileManager({
  sessionId,
  initialPath,
  className,
  canWrite = true,
  currentUserId,
  currentUserName,
  participants,
  onOpenFile,
  showTransfers = true,
}: FileManagerProps) {
  const queryClient = useQueryClient()
  const currentUserLabel = useMemo(
    () =>
      resolveParticipantName(participants ?? {}, currentUserId, currentUserName ?? currentUserId),
    [participants, currentUserId, currentUserName]
  )
  const participantMap = useMemo(() => participants ?? {}, [participants])

  const ensureSession = useSshWorkspaceStore((state) => state.ensureSession)
  const setBrowserPath = useSshWorkspaceStore((state) => state.setBrowserPath)
  const setShowHidden = useSshWorkspaceStore((state) => state.setShowHidden)
  const upsertTransfer = useSshWorkspaceStore((state) => state.upsertTransfer)
  const updateTransferState = useSshWorkspaceStore((state) => state.updateTransfer)
  const clearSessionTransfers = useSshWorkspaceStore((state) => state.clearCompletedTransfers)
  const cacheDirectory = useSshWorkspaceStore((state) => state.cacheDirectory)
  const getCachedDirectory = useSshWorkspaceStore((state) => state.getCachedDirectory)

  const initializedSessionRef = useRef<string | null>(null)
  useEffect(() => {
    if (initializedSessionRef.current !== sessionId) {
      ensureSession(sessionId)
      initializedSessionRef.current = sessionId
    }
  }, [ensureSession, sessionId])

  const sessionSlice = useSshWorkspaceStore((state) => state.sessions[sessionId])

  const browserPath = sessionSlice?.browserPath ?? '.'
  const showHidden = sessionSlice?.showHidden ?? false
  const transferOrder = sessionSlice?.transferOrder ?? EMPTY_TRANSFER_ORDER
  const transfersMap = sessionSlice?.transfers ?? EMPTY_TRANSFERS_MAP
  const cachedDirectory = useMemo(() => {
    if (!sessionId) {
      return undefined
    }
    return getCachedDirectory(sessionId, browserPath)
  }, [browserPath, getCachedDirectory, sessionId])

  useEffect(() => {
    if (initialPath) {
      const normalizedInitial = normalizePath(initialPath)
      if (normalizedInitial !== '.' && browserPath === '.') {
        setBrowserPath(sessionId, normalizedInitial)
      }
    }
  }, [browserPath, initialPath, sessionId, setBrowserPath])

  const [pathInput, setPathInput] = useState(() => displayPath(browserPath))

  useEffect(() => {
    setPathInput(displayPath(browserPath))
  }, [browserPath])

  const transfers = useMemo(
    () =>
      transferOrder
        .map((id) => transfersMap[id])
        .filter((transfer): transfer is TransferItem => Boolean(transfer)),
    [transferOrder, transfersMap]
  )

  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data, isLoading, error, refetch } = useSftpDirectory(sessionId, browserPath, {
    initialData: cachedDirectory,
    staleTime: cachedDirectory ? 15_000 : undefined,
  })
  const uploadMutation = useSftpUpload(sessionId)
  const deleteFileMutation = useSftpDeleteFile(sessionId)
  const deleteDirectoryMutation = useSftpDeleteDirectory(sessionId)

  const entries = useMemo(() => {
    if (!data?.entries) {
      return []
    }
    const filtered = showHidden
      ? data.entries
      : data.entries.filter((entry) => !entry.name.startsWith('.'))
    return sortEntries(filtered)
  }, [data?.entries, showHidden])

  const navigateTo = useCallback(
    (path: string) => {
      setBrowserPath(sessionId, path)
    },
    [sessionId, setBrowserPath]
  )

  const handleRefresh = useCallback(() => {
    void refetch()
  }, [refetch])

  const handlePathSubmit = useCallback(
    (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      navigateTo(pathInput)
    },
    [navigateTo, pathInput]
  )

  const handleGoUp = useCallback(() => {
    navigateTo(parentPath(browserPath))
  }, [browserPath, navigateTo])

  const handleDownload = useCallback(
    async (entry: SftpEntry) => {
      const transfer = createTransfer({
        name: entry.name,
        path: entry.path,
        direction: 'download',
        size: entry.size,
        userId: currentUserId,
        userName: currentUserLabel,
      })
      upsertTransfer(sessionId, { ...transfer, status: 'uploading' })

      try {
        const result = await sftpApi.download(sessionId, entry.path)
        const url = URL.createObjectURL(result.data)
        const link = document.createElement('a')
        link.href = url
        link.download = result.filename ?? entry.name
        document.body.appendChild(link)
        link.click()
        link.remove()
        URL.revokeObjectURL(url)

        updateTransferState(sessionId, transfer.id, (item) => ({
          ...item,
          size: result.size ?? item.size,
          uploaded: result.size ?? item.uploaded,
          status: 'completed',
          completedAt: new Date(),
        }))
        toast.success(`Downloading ${entry.name}`, {
          description: 'Download started in the background.',
        })
      } catch (err) {
        const apiError = toApiError(err)
        updateTransferState(sessionId, transfer.id, (item) => ({
          ...item,
          status: 'failed',
          errorMessage: apiError.message,
          completedAt: new Date(),
        }))
        toast.error(`Unable to download ${entry.name}`, {
          description: apiError.message,
        })
      }
    },
    [currentUserId, currentUserLabel, sessionId, upsertTransfer, updateTransferState]
  )

  const handleEntryActivate = useCallback(
    (entry: SftpEntry) => {
      if (entry.isDir) {
        navigateTo(entry.path)
        return
      }
      if (onOpenFile) {
        onOpenFile(entry)
        return
      }
      void handleDownload(entry)
    },
    [handleDownload, navigateTo, onOpenFile]
  )

  const handleUploadFiles = useCallback(
    async (files: FileList | File[] | null) => {
      if (!files || files.length === 0) {
        return
      }

      const iterable = Array.isArray(files) ? files : Array.from(files)

      for (const file of iterable) {
        const targetPath = resolveChildPath(browserPath, file.name)
        const transfer = createTransfer({
          name: file.name,
          path: targetPath,
          direction: 'upload',
          size: file.size,
          userId: currentUserId,
          userName: currentUserLabel,
        })
        upsertTransfer(sessionId, { ...transfer, status: 'uploading' })

        try {
          const result = await uploadMutation.mutateAsync({
            path: targetPath,
            blob: file,
            options: {
              createParents: true,
              onChunk: ({ uploadedBytes, totalBytes }) => {
                updateTransferState(sessionId, transfer.id, (item) => ({
                  ...item,
                  totalBytes: totalBytes > 0 ? totalBytes : item.totalBytes,
                  size: totalBytes > 0 ? totalBytes : item.size,
                  uploaded: totalBytes > 0 ? Math.min(uploadedBytes, totalBytes) : uploadedBytes,
                  status: 'uploading',
                }))
              },
            },
          })

          if (result?.transferId) {
            updateTransferState(sessionId, transfer.id, (item) => ({
              ...item,
              remoteId: result.transferId,
            }))
          }

          updateTransferState(sessionId, transfer.id, (item) => ({
            ...item,
            uploaded: item.size,
            status: 'completed',
            completedAt: new Date(),
          }))
          toast.success(`Uploaded ${file.name}`)
        } catch (err) {
          const apiError = toApiError(err)
          updateTransferState(sessionId, transfer.id, (item) => ({
            ...item,
            status: 'failed',
            errorMessage: apiError.message,
            completedAt: new Date(),
          }))
          toast.error(`Failed to upload ${file.name}`, {
            description: apiError.message,
          })
        }
      }

      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    },
    [
      browserPath,
      currentUserId,
      currentUserLabel,
      sessionId,
      upsertTransfer,
      updateTransferState,
      uploadMutation,
    ]
  )

  const handleRealtimeEvent = useCallback(
    (event: SftpTransferRealtimeEvent) => {
      const { payload, status } = event
      const state = useSshWorkspaceStore.getState()
      const session = state.sessions[sessionId]
      const now = new Date()

      let targetId: string | undefined
      if (session) {
        targetId = session.transferOrder.find(
          (id) => session.transfers[id]?.remoteId === payload.transferId
        )
        if (!targetId) {
          targetId = session.transferOrder.find((id) => {
            const item = session.transfers[id]
            return (
              item &&
              !item.remoteId &&
              item.path === payload.path &&
              item.direction === payload.direction
            )
          })
        }
      }

      const baseTransfer: TransferItem = {
        id: targetId ?? payload.transferId ?? `${payload.path}-${payload.direction}`,
        remoteId: payload.transferId,
        name: extractNameFromPath(payload.path),
        path: payload.path,
        direction: payload.direction,
        size: payload.totalBytes ?? 0,
        uploaded: payload.bytesTransferred ?? 0,
        status: status === 'failed' ? 'failed' : status === 'completed' ? 'completed' : 'uploading',
        startedAt: now,
        completedAt: status === 'completed' || status === 'failed' ? now : undefined,
        errorMessage: status === 'failed' ? payload.error : undefined,
        totalBytes: payload.totalBytes,
        userId: payload.userId,
        userName: resolveParticipantName(participantMap, payload.userId, payload.userId),
      }

      if (!targetId) {
        upsertTransfer(sessionId, baseTransfer)
        return
      }

      updateTransferState(sessionId, targetId, (existing) => {
        const next: TransferItem = {
          ...existing,
          remoteId: payload.transferId ?? existing.remoteId,
          name: existing.name || baseTransfer.name,
          path: payload.path || existing.path,
          direction: payload.direction || existing.direction,
          userId: payload.userId ?? existing.userId,
          userName: resolveParticipantName(
            participantMap,
            payload.userId,
            existing.userName ?? existing.userId
          ),
          totalBytes: payload.totalBytes ?? existing.totalBytes,
          size: payload.totalBytes ?? existing.size,
          uploaded:
            payload.bytesTransferred !== undefined ? payload.bytesTransferred : existing.uploaded,
          errorMessage: payload.error ?? existing.errorMessage,
          status:
            status === 'failed' ? 'failed' : status === 'completed' ? 'completed' : 'uploading',
          completedAt: status === 'completed' || status === 'failed' ? now : existing.completedAt,
        }
        if (status === 'completed' && payload.totalBytes !== undefined) {
          next.uploaded = payload.totalBytes
        }
        return next
      })
    },
    [participantMap, sessionId, upsertTransfer, updateTransferState]
  )

  useSftpTransfersStream({
    sessionId,
    enabled: Boolean(sessionId),
    onEvent: handleRealtimeEvent,
  })

  useEffect(() => {
    if (!data || !sessionId) {
      return
    }
    cacheDirectory(sessionId, data.path, data)
  }, [cacheDirectory, data, sessionId])

  useEffect(() => {
    if (!sessionId || !entries || entries.length === 0) {
      return
    }
    const directories = entries.filter((entry) => entry.isDir).slice(0, 5)
    directories.forEach((dir) => {
      const key = getSftpListQueryKey(sessionId, dir.path)
      void queryClient.prefetchQuery({
        queryKey: key,
        queryFn: () => sftpApi.list(sessionId, dir.path === '.' ? undefined : dir.path),
      })
    })
  }, [entries, queryClient, sessionId])

  const handleDeleteEntry = useCallback(
    async (entry: SftpEntry) => {
      if (!canWrite) {
        return
      }
      try {
        if (entry.isDir) {
          await deleteDirectoryMutation.mutateAsync({
            path: entry.path,
            options: { recursive: false },
          })
        } else {
          await deleteFileMutation.mutateAsync({ path: entry.path })
        }
        toast.success(`Deleted ${entry.name}`)
      } catch (err) {
        const apiError = toApiError(err)
        toast.error(`Unable to delete ${entry.name}`, {
          description: apiError.message,
        })
      }
    },
    [canWrite, deleteDirectoryMutation, deleteFileMutation]
  )

  const clearCompletedTransfers = useCallback(() => {
    clearSessionTransfers(sessionId)
  }, [clearSessionTransfers, sessionId])

  const renderEntryIcon = useCallback((entry: SftpEntry) => {
    if (entry.isDir) {
      return <Folder className="h-5 w-5 text-primary" aria-hidden />
    }
    return <FileText className="h-5 w-5 text-muted-foreground" aria-hidden />
  }, [])

  const { getRootProps, isDragActive, isDragReject, isDragAccept } = useDropzone({
    noClick: true,
    noKeyboard: true,
    disabled: !canWrite,
    multiple: true,
    onDropAccepted: (accepted) => {
      void handleUploadFiles(accepted)
    },
  })

  return (
    <PermissionGuard
      permission={PERMISSIONS.PROTOCOL.SSH.SFTP}
      fallback={
        <EmptyState
          title="Insufficient permissions"
          description="You do not have permission to access the remote file manager."
          className="h-full"
        />
      }
    >
      <div className={cn('flex h-full flex-col gap-4', className)}>
        <FileManagerToolbar
          isRootPath={browserPath === '.' || browserPath === '/'}
          isLoading={isLoading}
          showHidden={showHidden}
          onToggleHidden={(checked) => setShowHidden(sessionId, Boolean(checked))}
          onNavigateUp={handleGoUp}
          onNavigateHome={() => navigateTo('.')}
          onRefresh={handleRefresh}
          pathInput={pathInput}
          onPathInputChange={(value) => setPathInput(value)}
          onSubmitPath={handlePathSubmit}
          navigateUpLabel={
            <>
              <ArrowUp className="mr-2 h-4 w-4" aria-hidden />
              Up
            </>
          }
          navigateHomeLabel={
            <>
              <Home className="mr-2 h-4 w-4" aria-hidden />
              Home
            </>
          }
          refreshLabel={
            <>
              <RefreshCcw className="mr-2 h-4 w-4" aria-hidden />
              Refresh
            </>
          }
          uploadControl={
            <>
              <PermissionGuard permission={PERMISSIONS.PROTOCOL.SSH.SFTP}>
                <Button
                  variant="default"
                  size="sm"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={!canWrite || uploadMutation.isPending}
                >
                  {uploadMutation.isPending ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
                  ) : (
                    <Upload className="mr-2 h-4 w-4" aria-hidden />
                  )}
                  Upload
                </Button>
              </PermissionGuard>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                className="hidden"
                data-testid="sftp-upload-input"
                onChange={(event) => handleUploadFiles(event.target.files)}
                disabled={!canWrite}
              />
            </>
          }
        />

        <div className="flex flex-1 gap-4 overflow-hidden">
          <div {...getRootProps({ className: 'relative flex-1 overflow-hidden', tabIndex: -1 })}>
            <Card className="flex h-full flex-col overflow-hidden">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-sm font-semibold text-muted-foreground">
                  {entries.length === 1 ? '1 item' : `${entries.length} items`}
                </h3>
              </div>

              <div className="relative flex-1 overflow-auto">
                {isLoading ? (
                  <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
                    <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
                    Loading directory…
                  </div>
                ) : error ? (
                  <EmptyState
                    title="Unable to load directory"
                    description={toApiError(error).message}
                    className="m-6 min-h-[240px]"
                    action={
                      <Button size="sm" variant="secondary" onClick={handleRefresh}>
                        Retry
                      </Button>
                    }
                  />
                ) : entries.length === 0 ? (
                  <EmptyState
                    title="Folder is empty"
                    description="Upload files or navigate to another directory."
                    className="m-6 min-h-[240px]"
                  />
                ) : (
                  <FileManagerTable
                    entries={entries}
                    onActivate={handleEntryActivate}
                    onDownload={handleDownload}
                    onDelete={handleDeleteEntry}
                    canWrite={canWrite}
                    renderIcon={renderEntryIcon}
                  />
                )}

                <div
                  data-testid="sftp-dropzone-overlay"
                  className={cn(
                    'pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed border-primary/60 bg-primary/5 text-center text-sm font-semibold text-primary opacity-0 transition-opacity',
                    isDragActive && 'opacity-100',
                    isDragReject && 'border-destructive text-destructive'
                  )}
                >
                  {isDragReject ? (
                    <>
                      <p>Unsupported files</p>
                      <p className="text-xs text-destructive/80">
                        Please drop regular files or folders only.
                      </p>
                    </>
                  ) : (
                    <>
                      <p>{isDragAccept ? 'Release to upload' : 'Drop files to upload'}</p>
                      <p className="text-xs text-primary/80">
                        Files will upload to {displayPath(browserPath)}
                      </p>
                    </>
                  )}
                </div>
              </div>
            </Card>
          </div>

          {showTransfers && (
            <TransferSidebar transfers={transfers} onClear={clearCompletedTransfers} />
          )}
        </div>
      </div>
    </PermissionGuard>
  )
}

export default FileManager
