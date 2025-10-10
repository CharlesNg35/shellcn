import type { ApiResponse } from '@/types/api'
import type { ConnectionFolderSummary } from '@/types/connections'
import { apiClient } from './client'
import { unwrapResponse } from './http'

const FOLDER_ENDPOINT = '/connection-folders'

export interface ConnectionFolderNode {
  folder: ConnectionFolderSummary
  connection_count: number
  children?: ConnectionFolderNode[]
}

export async function fetchConnectionFolderTree(teamId?: string) {
  const params = teamId ? { team_id: teamId } : undefined
  const response = await apiClient.get<ApiResponse<ConnectionFolderNode[]>>(
    `${FOLDER_ENDPOINT}/tree`,
    {
      params,
    }
  )
  return unwrapResponse(response)
}

export interface UpsertFolderPayload {
  name: string
  description?: string
  icon?: string
  color?: string
  parent_id?: string | null
  team_id?: string | null
  metadata?: Record<string, unknown>
  ordering?: number
}

export async function createConnectionFolder(payload: UpsertFolderPayload) {
  const response = await apiClient.post<ApiResponse<ConnectionFolderSummary>>(
    FOLDER_ENDPOINT,
    payload
  )
  return unwrapResponse(response)
}

export async function updateConnectionFolder(id: string, payload: UpsertFolderPayload) {
  const response = await apiClient.patch<ApiResponse<ConnectionFolderSummary>>(
    `${FOLDER_ENDPOINT}/${id}`,
    payload
  )
  return unwrapResponse(response)
}

export async function deleteConnectionFolder(id: string) {
  const response = await apiClient.delete<ApiResponse<{ deleted: boolean }>>(
    `${FOLDER_ENDPOINT}/${id}`
  )
  return unwrapResponse(response)
}
