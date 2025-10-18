import { useCallback, useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type InvalidateQueryFilters,
  type UseMutationOptions,
  type UseMutationResult,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { sftpApi } from '@/lib/api/sftp'
import { ApiError } from '@/lib/api/http'
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

const SFTP_QUERY_ROOT = ['sftp'] as const

function normalizePath(path?: string): string {
  const trimmed = path?.trim()
  return trimmed && trimmed !== '' ? trimmed : '.'
}

function getListBaseKey(sessionId: string) {
  return [...SFTP_QUERY_ROOT, 'list', sessionId] as const
}

export function getSftpListQueryKey(sessionId: string, path?: string) {
  return [...getListBaseKey(sessionId), normalizePath(path)] as const
}

export function getSftpMetadataQueryKey(sessionId: string, path: string) {
  return [...SFTP_QUERY_ROOT, 'metadata', sessionId, normalizePath(path)] as const
}

export function getSftpFileQueryKey(sessionId: string, path: string) {
  return [...SFTP_QUERY_ROOT, 'file', sessionId, normalizePath(path)] as const
}

type DirectoryQueryOptions = Omit<UseQueryOptions<SftpListResult, ApiError>, 'queryKey' | 'queryFn'>

export function useSftpDirectory(
  sessionId: string | undefined,
  path?: string,
  options?: DirectoryQueryOptions
) {
  const normalizedPath = useMemo(() => normalizePath(path), [path])
  const queryKey = useMemo(() => {
    if (!sessionId) {
      return [...SFTP_QUERY_ROOT, 'list', 'unknown'] as const
    }
    return getSftpListQueryKey(sessionId, normalizedPath)
  }, [sessionId, normalizedPath])

  const { enabled: enabledOption = true, ...rest } = options ?? {}

  return useQuery<SftpListResult, ApiError>({
    queryKey,
    queryFn: () => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.list(sessionId, normalizedPath === '.' ? undefined : normalizedPath)
    },
    enabled: Boolean(sessionId) && enabledOption,
    ...rest,
  })
}

type MetadataQueryOptions = Omit<UseQueryOptions<SftpEntry, ApiError>, 'queryKey' | 'queryFn'>

export function useSftpMetadata(
  sessionId: string | undefined,
  path: string | undefined,
  options?: MetadataQueryOptions
) {
  const normalizedPath = useMemo(() => normalizePath(path), [path])
  const queryKey = useMemo(() => {
    if (!sessionId || !path) {
      return [...SFTP_QUERY_ROOT, 'metadata', 'unknown'] as const
    }
    return getSftpMetadataQueryKey(sessionId, normalizedPath)
  }, [sessionId, normalizedPath, path])

  const { enabled: enabledOption = true, ...rest } = options ?? {}

  return useQuery<SftpEntry, ApiError>({
    queryKey,
    queryFn: () => {
      if (!sessionId || !path) {
        throw new Error('sessionId and path are required')
      }
      return sftpApi.metadata(sessionId, normalizedPath)
    },
    enabled: Boolean(sessionId && path) && enabledOption,
    staleTime: 10_000,
    ...rest,
  })
}

type FileQueryOptions = Omit<UseQueryOptions<SftpFileContent, ApiError>, 'queryKey' | 'queryFn'>

export function useSftpFileContent(
  sessionId: string | undefined,
  path: string | undefined,
  options?: FileQueryOptions
) {
  const normalizedPath = useMemo(() => normalizePath(path), [path])
  const queryKey = useMemo(() => {
    if (!sessionId || !path) {
      return [...SFTP_QUERY_ROOT, 'file', 'unknown'] as const
    }
    return getSftpFileQueryKey(sessionId, normalizedPath)
  }, [sessionId, normalizedPath, path])

  const { enabled: enabledOption = true, ...rest } = options ?? {}

  return useQuery<SftpFileContent, ApiError>({
    queryKey,
    queryFn: () => {
      if (!sessionId || !path) {
        throw new Error('sessionId and path are required')
      }
      return sftpApi.readFile(sessionId, normalizedPath)
    },
    enabled: Boolean(sessionId && path) && enabledOption,
    ...rest,
  })
}

function useInvalidateSftpList(sessionId: string | undefined) {
  const queryClient = useQueryClient()
  return useCallback(
    (path?: string) => {
      if (!sessionId) {
        return
      }
      const baseKey = getListBaseKey(sessionId)
      const baseFilters: InvalidateQueryFilters = {
        queryKey: baseKey,
        exact: false,
      }
      void queryClient.invalidateQueries(baseFilters)
      if (path) {
        let dirPath = '.'
        if (path !== '.') {
          const lastSlash = path.lastIndexOf('/')
          if (lastSlash > 0) {
            dirPath = path.slice(0, lastSlash)
          }
        }
        dirPath = normalizePath(dirPath)
        const dirFilters: InvalidateQueryFilters = {
          queryKey: getSftpListQueryKey(sessionId, dirPath),
        }
        void queryClient.invalidateQueries(dirFilters)
      }
    },
    [queryClient, sessionId]
  )
}

type UploadVariables = {
  path: string
  blob: Blob
  options?: SftpUploadOptions
}

export function useSftpUpload(
  sessionId: string | undefined,
  options?: UseMutationOptions<SftpUploadResult, ApiError, UploadVariables>
): UseMutationResult<SftpUploadResult, ApiError, UploadVariables> {
  const invalidate = useInvalidateSftpList(sessionId)

  return useMutation<SftpUploadResult, ApiError, UploadVariables>({
    mutationFn: ({ path, blob, options: uploadOptions }) => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.upload(sessionId, path, blob, uploadOptions)
    },
    onSuccess: (result, variables, context, mutation) => {
      invalidate(variables.path)
      options?.onSuccess?.(result, variables, context, mutation)
    },
    ...options,
  })
}

type DeleteFileVariables = {
  path: string
}

export function useSftpDeleteFile(
  sessionId: string | undefined,
  options?: UseMutationOptions<void, ApiError, DeleteFileVariables>
): UseMutationResult<void, ApiError, DeleteFileVariables> {
  const invalidate = useInvalidateSftpList(sessionId)

  return useMutation<void, ApiError, DeleteFileVariables>({
    mutationFn: ({ path }) => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.deleteFile(sessionId, path)
    },
    onSuccess: (result, variables, context, mutation) => {
      invalidate(variables.path)
      options?.onSuccess?.(result, variables, context, mutation)
    },
    ...options,
  })
}

type DeleteDirectoryVariables = {
  path: string
  options?: SftpDeleteDirectoryOptions
}

export function useSftpDeleteDirectory(
  sessionId: string | undefined,
  options?: UseMutationOptions<void, ApiError, DeleteDirectoryVariables>
): UseMutationResult<void, ApiError, DeleteDirectoryVariables> {
  const invalidate = useInvalidateSftpList(sessionId)

  return useMutation<void, ApiError, DeleteDirectoryVariables>({
    mutationFn: ({ path, options: deleteOptions }) => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.deleteDirectory(sessionId, path, deleteOptions)
    },
    onSuccess: (result, variables, context, mutation) => {
      invalidate(variables.path)
      options?.onSuccess?.(result, variables, context, mutation)
    },
    ...options,
  })
}

type RenameVariables = SftpRenamePayload

export function useSftpRename(
  sessionId: string | undefined,
  options?: UseMutationOptions<{ source: string; target: string }, ApiError, RenameVariables>
) {
  const invalidate = useInvalidateSftpList(sessionId)

  return useMutation<{ source: string; target: string }, ApiError, RenameVariables>({
    mutationFn: (payload) => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.rename(sessionId, payload)
    },
    onSuccess: (result, variables, context, mutation) => {
      invalidate(result.target)
      options?.onSuccess?.(result, variables, context, mutation)
    },
    ...options,
  })
}

type SaveFileVariables = SftpSavePayload

export function useSftpSaveFile(
  sessionId: string | undefined,
  options?: UseMutationOptions<SftpEntry, ApiError, SaveFileVariables>
) {
  const invalidate = useInvalidateSftpList(sessionId)

  return useMutation<SftpEntry, ApiError, SaveFileVariables>({
    mutationFn: (payload) => {
      if (!sessionId) {
        throw new Error('sessionId is required')
      }
      return sftpApi.saveFile(sessionId, payload)
    },
    onSuccess: (result, variables, context, mutation) => {
      invalidate(result.path)
      options?.onSuccess?.(result, variables, context, mutation)
    },
    ...options,
  })
}
