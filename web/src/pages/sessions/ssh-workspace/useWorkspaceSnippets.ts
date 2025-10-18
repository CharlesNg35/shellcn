import { useCallback, useMemo } from 'react'
import { toast } from '@/lib/utils/toast'
import { useSnippets, useExecuteSnippet } from '@/hooks/useSnippets'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SnippetRecord } from '@/lib/api/snippets'
import type { SshSnippetGroup } from '@/components/workspace/ssh/SshWorkspaceToolbar'

interface UseWorkspaceSnippetsParams {
  session?: ActiveConnectionSession
  enabled: boolean
  logEvent: (action: string, details?: Record<string, unknown>) => void
}

interface UseWorkspaceSnippetsResult {
  groups: SshSnippetGroup[]
  isLoading: boolean
  snippetsAvailable: boolean
  executeSnippet: (snippetId: string) => void
  isExecuting: boolean
}

export function useWorkspaceSnippets({
  session,
  enabled,
  logEvent,
}: UseWorkspaceSnippetsParams): UseWorkspaceSnippetsResult {
  const snippetsQuery = useSnippets({
    enabled: enabled && Boolean(session),
    scope: 'all',
    connectionId: session?.connection_id,
  })

  const snippets = useMemo(() => snippetsQuery.data ?? [], [snippetsQuery.data])
  const groups = useMemo(() => groupSnippets(snippets), [snippets])
  const snippetsAvailable = groups.some((group) => group.snippets.length > 0)

  const executeSnippetMutation = useExecuteSnippet({
    onSuccess: () => {
      toast.success('Snippet executed')
      logEvent('snippet.execute.success', { sessionId: session?.id })
    },
    onError: (error) => {
      toast.error('Failed to execute snippet', { description: error.message })
      logEvent('snippet.execute.error', { sessionId: session?.id, error: error.message })
    },
  })

  const executeSnippet = useCallback(
    (snippetId: string) => {
      if (!session) {
        return
      }
      executeSnippetMutation.mutate({ sessionId: session.id, snippetId })
    },
    [executeSnippetMutation, session]
  )

  return {
    groups,
    isLoading: snippetsQuery.isLoading,
    snippetsAvailable,
    executeSnippet,
    isExecuting: executeSnippetMutation.isPending,
  }
}

function groupSnippets(snippets: SnippetRecord[]): SshSnippetGroup[] {
  const groups: SshSnippetGroup[] = []
  const byScope = new Map<SnippetRecord['scope'], SnippetRecord[]>()
  snippets.forEach((snippet) => {
    const scopeGroup = byScope.get(snippet.scope) ?? []
    scopeGroup.push(snippet)
    byScope.set(snippet.scope, scopeGroup)
  })

  const scopeOrder: Array<{ scope: SnippetRecord['scope']; label: string }> = [
    { scope: 'global', label: 'Global snippets' },
    { scope: 'connection', label: 'Connection snippets' },
    { scope: 'user', label: 'Personal snippets' },
  ]

  scopeOrder.forEach(({ scope, label }) => {
    const bucket = byScope.get(scope)
    if (bucket?.length) {
      const sorted = [...bucket].sort((a, b) => a.name.localeCompare(b.name))
      groups.push({
        label,
        snippets: sorted.map((snippet) => ({
          id: snippet.id,
          name: snippet.name,
          description: snippet.description,
        })),
      })
    }
  })

  return groups
}
