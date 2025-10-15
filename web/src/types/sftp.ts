export type SftpEntryType = 'file' | 'directory' | 'symlink'

export interface SftpEntry {
  name: string
  path: string
  type: SftpEntryType
  isDir: boolean
  size: number
  mode: string
  modifiedAt: Date
}

export interface SftpListResult {
  path: string
  entries: SftpEntry[]
}

export interface SftpFileContent {
  entry: SftpEntry
  encoding: string
  content: string
}

export interface SftpRenamePayload {
  source: string
  target: string
  overwrite?: boolean
}

export interface SftpSavePayload {
  path: string
  content: string
  encoding?: string
  createParents?: boolean
}

export interface SftpUploadOptions {
  createParents?: boolean
  append?: boolean
  chunkSize?: number
  offset?: number
  signal?: AbortSignal
  onChunk?: (context: {
    uploadedBytes: number
    totalBytes: number
    chunkBytes: number
    nextOffset: number
    transferId?: string
  }) => void
}

export interface SftpDeleteDirectoryOptions {
  recursive?: boolean
}

export interface SftpUploadResult {
  path: string
  uploadedBytes: number
  nextOffset: number
  transferId?: string
}

export type SftpTransferDirection = 'upload' | 'download' | 'save' | (string & {})
export type SftpTransferStatus = 'started' | 'progress' | 'completed' | 'failed' | (string & {})

export interface SftpTransferEventPayload {
  sessionId: string
  connectionId?: string
  userId?: string
  path: string
  direction: SftpTransferDirection
  transferId: string
  status: SftpTransferStatus
  bytesTransferred?: number
  totalBytes?: number
  error?: string
}

export interface SftpTransferRealtimeEvent {
  event: string
  status: SftpTransferStatus
  payload: SftpTransferEventPayload
}
