import type { ApiResponse } from '@/types/api'
import { apiClient } from './client'
import { unwrapResponse } from './http'

export type SnippetScope = 'global' | 'connection' | 'user'

export interface SnippetRecord {
  id: string
  name: string
  description?: string
  command: string
  scope: SnippetScope
  owner_id?: string | null
  connection_id?: string | null
  updated_at?: string | null
}

export interface FetchSnippetsParams {
  scope?: SnippetScope | 'all'
  connectionId?: string
}

export async function fetchSnippets(params: FetchSnippetsParams = {}): Promise<SnippetRecord[]> {
  const { scope, connectionId } = params
  const query = new URLSearchParams()
  if (scope && scope !== 'all') {
    query.set('scope', scope)
  }
  if (connectionId) {
    query.set('connection_id', connectionId)
  }

  const endpoint = query.toString() ? `/snippets?${query.toString()}` : '/snippets'
  const response = await apiClient.get<ApiResponse<SnippetRecord[]>>(endpoint)
  return unwrapResponse(response)
}

interface ExecuteSnippetPayload {
  snippet_id: string
}

export async function executeSnippet(sessionId: string, snippetId: string): Promise<void> {
  const response = await apiClient.post<ApiResponse<unknown>>(
    `/active-sessions/${encodeURIComponent(sessionId)}/snippet`,
    { snippet_id: snippetId } satisfies ExecuteSnippetPayload
  )
  unwrapResponse(response)
}
