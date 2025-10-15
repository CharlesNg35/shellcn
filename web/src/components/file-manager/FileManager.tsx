import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowUp,
  Download,
  FileText,
  Folder,
  Home,
  Loader2,
  MoreVertical,
  RefreshCcw,
  Trash2,
  Upload,
} from 'lucide-react'
import { format } from 'date-fns'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Checkbox } from '@/components/ui/Checkbox'
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
import type { SftpEntry } from '@/types/sftp'

interface FileManagerProps {
  sessionId: string
  initialPath?: string
  className?: string
  canWrite?: boolean
}

type TransferStatus = 'pending' | 'uploading' | 'completed' | 'failed'

interface TransferItem {
  id: string
  name: string
  size: number
  uploaded: number
  status: TransferStatus
  startedAt: Date
  completedAt?: Date
  errorMessage?: string
}

function normalizePath(path?: string): string {
  const trimmed = path?.trim()
  if (!trimmed || trimmed === '.' || trimmed === '/') {
    return '.'
  }
  return trimmed.replace(/^\/+/, '').replace(/\/+$/, '')
}

function displayPath(path: string): string {
  if (!path || path === '.' || path === '/') {
    return '/'
  }
  return path.startsWith('/') ? path : `/${path}`
}

function resolveChildPath(basePath: string, name: string): string {
  const safeName = name.replace(/^\//, '')
  if (!basePath || basePath === '.' || basePath === '/') {
    return safeName
  }
  return `${basePath.replace(/\/+$/, '')}/${safeName}`
}

function parentPath(path: string): string {
  if (!path || path === '.' || path === '/') {
    return '.'
  }
  const normalized = path.replace(/\/+$/, '')
  const slashIndex = normalized.lastIndexOf('/')
  if (slashIndex <= 0) {
    return '.'
  }
  return normalized.slice(0, slashIndex)
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  const absValue = Math.abs(value)
  if (absValue < 1024) {
    return `${value} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let index = -1
  let size = absValue
  do {
    size /= 1024
    index += 1
  } while (size >= 1024 && index < units.length - 1)
  const formatted = `${value < 0 ? '-' : ''}${size.toFixed(size >= 10 ? 0 : 1)} ${units[index]}`
  return formatted
}

function sortEntries(entries: SftpEntry[]): SftpEntry[] {
  return [...entries].sort((a, b) => {
    if (a.isDir && !b.isDir) {
      return -1
    }
    if (!a.isDir && b.isDir) {
      return 1
    }
    return a.name.localeCompare(b.name)
  })
}

function createTransfer(file: File): TransferItem {
  return {
    id: crypto.randomUUID ? crypto.randomUUID() : `${file.name}-${Date.now()}`,
    name: file.name,
    size: file.size,
    uploaded: 0,
    status: 'pending',
    startedAt: new Date(),
  }
}

export function FileManager({
  sessionId,
  initialPath,
  className,
  canWrite = true,
}: FileManagerProps) {
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

  const handleDownload = useCallback(
    async (entry: SftpEntry) => {
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
        toast.success(`Downloading ${entry.name}`, {
          description: 'Download started in the background.',
        })
      } catch (err) {
        const apiError = toApiError(err)
        toast.error(`Unable to download ${entry.name}`, {
          description: apiError.message,
        })
      }
    },
    [sessionId]
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

  const handleUploadFiles = useCallback(
    async (files: FileList | null) => {
      if (!files || files.length === 0) {
        return
      }

      for (const file of Array.from(files)) {
        const transfer = createTransfer(file)
        setTransfers((items) => [...items, { ...transfer, status: 'uploading' }])

        try {
          await uploadMutation.mutateAsync({
            path: resolveChildPath(currentPath, file.name),
            blob: file,
            options: {
              createParents: true,
              onChunk: ({ uploadedBytes, totalBytes }) => {
                updateTransfer(transfer.id, (item) => ({
                  ...item,
                  uploaded: totalBytes === 0 ? uploadedBytes : Math.min(uploadedBytes, totalBytes),
                  status: 'uploading',
                }))
              },
            },
          })

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
    [currentPath, updateTransfer, uploadMutation]
  )

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
        <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="ghost" size="sm" onClick={handleGoUp} disabled={currentPath === '.'}>
              <ArrowUp className="mr-2 h-4 w-4" aria-hidden />
              Up
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => navigateTo('.')}
              disabled={currentPath === '.'}
            >
              <Home className="mr-2 h-4 w-4" aria-hidden />
              Home
            </Button>
            <Button variant="ghost" size="sm" onClick={handleRefresh} disabled={isLoading}>
              <RefreshCcw className="mr-2 h-4 w-4" aria-hidden />
              Refresh
            </Button>
            <div className="ml-auto flex items-center gap-2">
              <label className="flex items-center gap-2 text-sm text-muted-foreground">
                <Checkbox
                  checked={showHidden}
                  onCheckedChange={(checked) => setShowHidden(Boolean(checked))}
                />
                Show hidden files
              </label>
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
            </div>
          </div>
          <form className="flex items-center gap-3" onSubmit={handlePathSubmit}>
            <label
              className="text-xs font-semibold uppercase text-muted-foreground"
              htmlFor="sftp-path"
            >
              Current path
            </label>
            <Input
              id="sftp-path"
              value={pathInput}
              onChange={(event) => setPathInput(event.target.value)}
              className="flex-1"
              autoComplete="off"
            />
            <Button type="submit" size="sm" variant="secondary">
              Go
            </Button>
          </form>
        </div>

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
                  <table className="min-w-full text-sm">
                    <thead className="sticky top-0 z-10 bg-muted/70 backdrop-blur">
                      <tr className="text-left">
                        <th className="px-4 py-2 font-medium text-muted-foreground">Name</th>
                        <th className="px-4 py-2 font-medium text-muted-foreground">Size</th>
                        <th className="px-4 py-2 font-medium text-muted-foreground">Modified</th>
                        <th className="px-4 py-2 font-medium text-muted-foreground">Mode</th>
                        <th className="px-4 py-2 text-right font-medium text-muted-foreground">
                          Actions
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {entries.map((entry) => (
                        <tr
                          key={entry.path}
                          className="group cursor-pointer border-b border-border/80 hover:bg-muted/40"
                          onDoubleClick={() => handleEntryActivate(entry)}
                        >
                          <td className="flex items-center gap-3 px-4 py-2">
                            {renderEntryIcon(entry)}
                            <div className="flex flex-col">
                              <span className="font-medium text-foreground">{entry.name}</span>
                              <span className="text-xs text-muted-foreground">
                                {displayPath(entry.path)}
                              </span>
                            </div>
                          </td>
                          <td className="px-4 py-2 text-muted-foreground">
                            {entry.isDir ? '—' : formatBytes(entry.size)}
                          </td>
                          <td className="px-4 py-2 text-muted-foreground">
                            {format(entry.modifiedAt, 'yyyy-MM-dd HH:mm')}
                          </td>
                          <td className="px-4 py-2 text-muted-foreground">{entry.mode}</td>
                          <td className="px-4 py-2">
                            <div className="flex justify-end gap-1 opacity-0 transition group-hover:opacity-100">
                              <Button
                                variant="ghost"
                                size="icon"
                                aria-label="Download"
                                onClick={(event) => {
                                  event.stopPropagation()
                                  void handleDownload(entry)
                                }}
                                disabled={entry.isDir}
                              >
                                <Download className="h-4 w-4" aria-hidden />
                              </Button>
                              {canWrite && (
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  aria-label="Delete"
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleDeleteEntry(entry)
                                  }}
                                >
                                  <Trash2 className="h-4 w-4 text-destructive" aria-hidden />
                                </Button>
                              )}
                              <Button
                                variant="ghost"
                                size="icon"
                                aria-label="More actions"
                                disabled
                              >
                                <MoreVertical
                                  className="h-4 w-4 text-muted-foreground"
                                  aria-hidden
                                />
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </Card>

          <aside className="w-full max-w-xs space-y-3 rounded-lg border border-border bg-card p-4 shadow-sm">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-muted-foreground">Transfers</h3>
              <Button
                variant="ghost"
                size="sm"
                onClick={clearCompletedTransfers}
                disabled={!transfers.some((transfer) => transfer.status !== 'uploading')}
              >
                Clear completed
              </Button>
            </div>

            {transfers.length === 0 ? (
              <div className="rounded-md border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
                No active transfers. Upload files to see progress here.
              </div>
            ) : (
              <ul className="space-y-2 overflow-y-auto">
                {transfers.map((transfer) => {
                  const progress = transfer.size
                    ? Math.min(transfer.uploaded / transfer.size, 1)
                    : transfer.uploaded > 0
                      ? 1
                      : 0
                  return (
                    <li
                      key={transfer.id}
                      className="rounded-md border border-border/80 bg-background/80 p-3 shadow-sm"
                    >
                      <div className="flex items-center justify-between text-sm font-medium">
                        <span className="truncate">{transfer.name}</span>
                        <span className="text-xs text-muted-foreground">
                          {formatBytes(transfer.size)}
                        </span>
                      </div>
                      <div className="mt-2 h-2 rounded-full bg-muted">
                        <div
                          className={cn('h-2 rounded-full bg-primary transition-all', {
                            'bg-destructive': transfer.status === 'failed',
                          })}
                          style={{ width: `${progress * 100}%` }}
                        />
                      </div>
                      <div className="mt-2 flex justify-between text-xs text-muted-foreground">
                        <span className="capitalize">{transfer.status}</span>
                        <span>
                          {transfer.uploaded === transfer.size
                            ? formatBytes(transfer.size)
                            : `${formatBytes(transfer.uploaded)} / ${formatBytes(transfer.size)}`}
                        </span>
                      </div>
                      {transfer.errorMessage && (
                        <p className="mt-2 text-xs text-destructive">{transfer.errorMessage}</p>
                      )}
                    </li>
                  )
                })}
              </ul>
            )}
          </aside>
        </div>
      </div>
    </PermissionGuard>
  )
}

export default FileManager
