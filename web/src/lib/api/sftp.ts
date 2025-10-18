import type { AxiosRequestConfig } from 'axios'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import type { ApiResponse } from '@/types/api'
import type {
  SftpDeleteDirectoryOptions,
  SftpEntry,
  SftpFileContent,
  SftpListResult,
  SftpRenamePayload,
  SftpSavePayload,
  SftpUploadOptions,
  SftpUploadResult,
} from '@/types/sftp'

interface SftpEntryResponse {
  name: string
  path: string
  type: string
  is_dir: boolean
  size: number
  mode: string
  modified_at: string
}

interface SftpListResponse {
  path: string
  entries: SftpEntryResponse[]
}

interface SftpFileContentResponse {
  entry: SftpEntryResponse
  encoding: string
  content: string
}

interface SftpUploadResponse {
  path: string
  bytes_written: number
  next_offset: number
  transfer_id?: string
}

function parseDate(value: string): Date {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return new Date()
  }
  return date
}

function mapEntry(entry: SftpEntryResponse): SftpEntry {
  let type: SftpEntry['type']
  if (entry.is_dir || entry.type === 'directory') {
    type = 'directory'
  } else if (entry.type === 'symlink') {
    type = 'symlink'
  } else {
    type = 'file'
  }
  return {
    name: entry.name,
    path: entry.path,
    type,
    isDir: entry.is_dir || type === 'directory',
    size: entry.size,
    mode: entry.mode,
    modifiedAt: parseDate(entry.modified_at),
  }
}

function mapListResponse(payload: SftpListResponse): SftpListResult {
  return {
    path: payload.path,
    entries: payload.entries.map(mapEntry),
  }
}

export async function listSftpEntries(sessionId: string, path?: string): Promise<SftpListResult> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }

  const response = await apiClient.get<ApiResponse<SftpListResponse>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/list`,
    {
      params: path ? { path } : undefined,
    }
  )

  const payload = unwrapResponse(response)
  return mapListResponse(payload)
}

export async function fetchSftpMetadata(sessionId: string, path: string): Promise<SftpEntry> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }

  const response = await apiClient.get<ApiResponse<SftpEntryResponse>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/metadata`,
    {
      params: { path },
    }
  )
  const payload = unwrapResponse(response)
  return mapEntry(payload)
}

export async function readSftpFile(sessionId: string, path: string): Promise<SftpFileContent> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }

  const response = await apiClient.get<ApiResponse<SftpFileContentResponse>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/file`,
    {
      params: { path },
    }
  )

  const payload = unwrapResponse(response)
  return {
    entry: mapEntry(payload.entry),
    encoding: payload.encoding,
    content: payload.content,
  }
}

export async function saveSftpFile(
  sessionId: string,
  payload: SftpSavePayload
): Promise<SftpEntry> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!payload?.path) {
    throw new Error('path is required')
  }

  const response = await apiClient.put<ApiResponse<SftpEntryResponse>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/file`,
    {
      path: payload.path,
      content: payload.content,
      encoding: payload.encoding ?? 'base64',
      create_parents: payload.createParents ?? false,
    }
  )

  return mapEntry(unwrapResponse(response))
}

export async function deleteSftpFile(sessionId: string, path: string): Promise<void> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }

  const response = await apiClient.delete<ApiResponse<Record<string, unknown>>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/file`,
    {
      params: { path },
    }
  )
  unwrapResponse(response)
}

export async function deleteSftpDirectory(
  sessionId: string,
  path: string,
  options?: SftpDeleteDirectoryOptions
): Promise<void> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }

  const response = await apiClient.delete<ApiResponse<Record<string, unknown>>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/directory`,
    {
      params: {
        path,
        recursive: options?.recursive ? 'true' : undefined,
      },
    }
  )
  unwrapResponse(response)
}

export async function renameSftpEntry(
  sessionId: string,
  payload: SftpRenamePayload
): Promise<{ source: string; target: string }> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!payload?.source || !payload?.target) {
    throw new Error('source and target are required')
  }

  const response = await apiClient.post<ApiResponse<{ source: string; target: string }>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/rename`,
    {
      source: payload.source,
      target: payload.target,
      overwrite: payload.overwrite ?? false,
    }
  )

  return unwrapResponse(response)
}

export interface SftpDownloadResult {
  data: Blob
  filename?: string
  contentType?: string
  size: number
  lastModified?: Date
}

function parseContentDisposition(header?: string): string | undefined {
  if (!header) {
    return undefined
  }
  const match = /filename\*?=([^;]+)/i.exec(header)
  if (!match) {
    return undefined
  }
  const value = match[1].trim()
  if (value.startsWith("UTF-8''")) {
    try {
      return decodeURIComponent(value.slice(7))
    } catch {
      return value.slice(7)
    }
  }
  return value.replace(/^"(.*)"$/, '$1')
}

export async function downloadSftpFile(
  sessionId: string,
  path: string,
  config?: Pick<AxiosRequestConfig, 'signal'>
): Promise<SftpDownloadResult> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }

  const response = await apiClient.get<ArrayBuffer>(
    `/active-sessions/${encodeURIComponent(sessionId)}/sftp/download`,
    {
      params: { path },
      responseType: 'arraybuffer',
      signal: config?.signal,
    }
  )

  const contentType = response.headers['content-type'] ?? undefined
  const contentDisposition = response.headers['content-disposition'] ?? undefined
  const lastModifiedHeader = response.headers['last-modified'] ?? undefined
  const filename = parseContentDisposition(contentDisposition)
  const lastModified = lastModifiedHeader ? new Date(lastModifiedHeader) : undefined

  return {
    data: new Blob([response.data], { type: contentType }),
    filename,
    contentType,
    size: response.data.byteLength,
    lastModified,
  }
}

export async function uploadSftpFile(
  sessionId: string,
  path: string,
  blob: Blob,
  options?: SftpUploadOptions
): Promise<SftpUploadResult> {
  if (!sessionId) {
    throw new Error('sessionId is required')
  }
  if (!path) {
    throw new Error('path is required')
  }
  if (!blob) {
    throw new Error('blob is required')
  }

  const chunkSize = options?.chunkSize ?? 16 * 1024 * 1024
  if (chunkSize <= 0) {
    throw new Error('chunkSize must be greater than zero')
  }

  let remoteOffset = options?.offset ?? 0
  let uploadedBytes = 0
  const totalBytes = blob.size
  let transferId: string | undefined

  while (uploadedBytes < totalBytes) {
    const nextSliceEnd = Math.min(uploadedBytes + chunkSize, totalBytes)
    const chunk = blob.slice(uploadedBytes, nextSliceEnd)

    const response = await apiClient.post<ApiResponse<SftpUploadResponse>>(
      `/active-sessions/${encodeURIComponent(sessionId)}/sftp/upload`,
      chunk,
      {
        headers: {
          'Content-Type': 'application/octet-stream',
          'Upload-Offset': remoteOffset.toString(),
        },
        params: {
          path,
          create_parents: options?.createParents ? 'true' : undefined,
          append: options?.append ? 'true' : undefined,
        },
        signal: options?.signal,
        maxBodyLength: Infinity,
        maxContentLength: Infinity,
      }
    )

    const payload = unwrapResponse(response)
    transferId = payload.transfer_id ?? transferId

    uploadedBytes += payload.bytes_written
    remoteOffset = payload.next_offset

    options?.onChunk?.({
      uploadedBytes,
      totalBytes,
      chunkBytes: payload.bytes_written,
      nextOffset: remoteOffset,
      transferId,
    })
  }

  return {
    path,
    uploadedBytes,
    nextOffset: remoteOffset,
    transferId,
  }
}

export const sftpApi = {
  list: listSftpEntries,
  metadata: fetchSftpMetadata,
  readFile: readSftpFile,
  saveFile: saveSftpFile,
  deleteFile: deleteSftpFile,
  deleteDirectory: deleteSftpDirectory,
  rename: renameSftpEntry,
  download: downloadSftpFile,
  upload: uploadSftpFile,
}
