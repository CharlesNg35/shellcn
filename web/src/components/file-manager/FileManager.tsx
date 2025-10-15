import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowUp, FileText, Folder, Home, Loader2, RefreshCcw, Upload } from 'lucide-react'
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

interface FileManagerProps {
  sessionId: string
  initialPath?: string
  className?: string
  canWrite?: boolean
  currentUserId?: string
  currentUserName?: string
  participants?: Record<string, ActiveSessionParticipant>
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
}: FileManagerProps) {
  const currentUserLabel = useMemo(
    () =>
      resolveParticipantName(participants ?? {}, currentUserId, currentUserName ?? currentUserId),
    [participants, currentUserId, currentUserName]
  )
  const participantMap = useMemo(() => participants ?? {}, [participants])
  const [currentPath, setCurrentPath] = useState(() => normalizePath(initialPath))
  const [pathInput, setPathInput] = useState(() => displayPath(normalizePath(initialPath)))
  const [showHidden, setShowHidden] = useState(false)
  const [transfers, setTransfers] = useState<TransferItem[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setPathInput(displayPath(currentPath))
  }, [currentPath])

  const { data, isLoading, error, refetch } = useSftpDirectory(sessionId, currentPath)
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

  const navigateTo = useCallback((path: string) => {
    setCurrentPath(normalizePath(path))
  }, [])

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
    navigateTo(parentPath(currentPath))
  }, [currentPath, navigateTo])

  const updateTransfer = useCallback(
    (id: string, updater: (transfer: TransferItem) => TransferItem) => {
      setTransfers((items) =>
        items.map((item) => {
          if (item.id !== id) {
            return item
          }
          return updater(item)
        })
      )
    },
    []
  )

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
      setTransfers((items) => [...items, { ...transfer, status: 'uploading' }])

      try {
        const result = await import('@/lib/api/sftp').then((module) =>
          module.downloadSftpFile(sessionId, entry.path)
        )
        const url = URL.createObjectURL(result.data)
        const link = document.createElement('a')
        link.href = url
        link.download = result.filename ?? entry.name
        document.body.appendChild(link)
        link.click()
        link.remove()
        URL.revokeObjectURL(url)

        updateTransfer(transfer.id, (item) => ({
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
        updateTransfer(transfer.id, (item) => ({
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
    [currentUserId, currentUserLabel, sessionId, updateTransfer]
  )

  const handleEntryActivate = useCallback(
    (entry: SftpEntry) => {
      if (entry.isDir) {
        navigateTo(entry.path)
        return
      }
      void handleDownload(entry)
    },
    [handleDownload, navigateTo]
  )

  const handleUploadFiles = useCallback(
    async (files: FileList | null) => {
      if (!files || files.length === 0) {
        return
      }

      for (const file of Array.from(files)) {
        const targetPath = resolveChildPath(currentPath, file.name)
        const transfer = createTransfer({
          name: file.name,
          path: targetPath,
          direction: 'upload',
          size: file.size,
          userId: currentUserId,
          userName: currentUserLabel,
        })
        setTransfers((items) => [...items, { ...transfer, status: 'uploading' }])

        try {
          const result = await uploadMutation.mutateAsync({
            path: targetPath,
            blob: file,
            options: {
              createParents: true,
              onChunk: ({ uploadedBytes, totalBytes }) => {
                updateTransfer(transfer.id, (item) => ({
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
            setTransfers((items) =>
              items.map((item) =>
                item.id === transfer.id ? { ...item, remoteId: result.transferId } : item
              )
            )
          }

          updateTransfer(transfer.id, (item) => ({
            ...item,
            uploaded: item.size,
            status: 'completed',
            completedAt: new Date(),
          }))
          toast.success(`Uploaded ${file.name}`)
        } catch (err) {
          const apiError = toApiError(err)
          updateTransfer(transfer.id, (item) => ({
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
    [currentPath, currentUserId, currentUserLabel, updateTransfer, uploadMutation]
  )

  const handleRealtimeEvent = useCallback(
    (event: SftpTransferRealtimeEvent) => {
      const { payload, status } = event
      setTransfers((items) => {
        let index = items.findIndex((item) => item.remoteId === payload.transferId)
        if (index < 0) {
          index = items.findIndex(
            (item) =>
              !item.remoteId && item.path === payload.path && item.direction === payload.direction
          )
        }

        const now = new Date()

        if (index < 0) {
          const newItem: TransferItem = {
            id: payload.transferId,
            remoteId: payload.transferId,
            name: extractNameFromPath(payload.path),
            path: payload.path,
            direction: payload.direction,
            size: payload.totalBytes ?? 0,
            uploaded: payload.bytesTransferred ?? 0,
            status:
              status === 'failed' ? 'failed' : status === 'completed' ? 'completed' : 'uploading',
            startedAt: now,
            completedAt: status === 'completed' || status === 'failed' ? now : undefined,
            errorMessage: status === 'failed' ? payload.error : undefined,
            totalBytes: payload.totalBytes,
            userId: payload.userId,
            userName: resolveParticipantName(participantMap, payload.userId, payload.userId),
          }
          const nextItems = [...items, newItem]
          return nextItems.length > 50 ? nextItems.slice(nextItems.length - 50) : nextItems
        }

        const nextItems = [...items]
        const current = { ...nextItems[index] }
        current.remoteId = payload.transferId
        current.name = current.name || extractNameFromPath(payload.path)
        current.path = payload.path
        current.direction = payload.direction
        if (payload.totalBytes !== undefined) {
          current.totalBytes = payload.totalBytes
          current.size = payload.totalBytes
        }
        if (payload.bytesTransferred !== undefined) {
          current.uploaded = payload.bytesTransferred
        }
        if (payload.userId) {
          current.userId = payload.userId
          current.userName = resolveParticipantName(participantMap, payload.userId, payload.userId)
        }

        if (status === 'completed') {
          current.status = 'completed'
          current.completedAt = now
          if (payload.totalBytes !== undefined) {
            current.uploaded = payload.totalBytes
          }
        } else if (status === 'failed') {
          current.status = 'failed'
          current.completedAt = now
          current.errorMessage = payload.error ?? current.errorMessage
        } else {
          current.status = 'uploading'
        }

        nextItems[index] = current
        return nextItems
      })
    },
    [participantMap]
  )

  useSftpTransfersStream({
    sessionId,
    enabled: Boolean(sessionId),
    onEvent: handleRealtimeEvent,
  })

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
    setTransfers((items) => items.filter((item) => item.status === 'uploading'))
  }, [])

  const renderEntryIcon = useCallback((entry: SftpEntry) => {
    if (entry.isDir) {
      return <Folder className="h-5 w-5 text-primary" aria-hidden />
    }
    return <FileText className="h-5 w-5 text-muted-foreground" aria-hidden />
  }, [])

  if (!sessionId) {
    return (
      <EmptyState
        title="SFTP unavailable"
        description="A valid session is required to browse remote files."
        className="h-full"
      />
    )
  }

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
          isRootPath={currentPath === '.'}
          isLoading={isLoading}
          showHidden={showHidden}
          onToggleHidden={(checked) => setShowHidden(checked)}
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
          <Card className="flex-1 overflow-hidden">
            <div className="flex h-full flex-col overflow-hidden">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-sm font-semibold text-muted-foreground">
                  {entries.length === 1 ? '1 item' : `${entries.length} items`}
                </h3>
              </div>

              <div className="flex-1 overflow-auto">
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
              </div>
            </div>
          </Card>

          <TransferSidebar transfers={transfers} onClear={clearCompletedTransfers} />
        </div>
      </div>
    </PermissionGuard>
  )
}

export default FileManager
