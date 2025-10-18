import { useCallback, useEffect, useMemo, useState } from 'react'
import { Loader2, RefreshCcw, RotateCcw, Save } from 'lucide-react'
import Editor from '@monaco-editor/react'
import { Button } from '@/components/ui/Button'
import { useSftpFileContent, useSftpSaveFile } from '@/hooks/useSftp'
import { toast } from '@/lib/utils/toast'
import { toApiError } from '@/lib/api/http'
import { useSshWorkspaceStore } from '@/store/ssh-workspace-store'
import { extractNameFromPath } from '../file-manager/utils'

interface SftpFileEditorProps {
  sessionId: string
  tabId: string
  path: string
  canWrite: boolean
}

function decodeBase64(value: string): string {
  if (typeof window === 'undefined') {
    return Buffer.from(value, 'base64').toString('utf-8')
  }
  return window.atob(value)
}

function encodeBase64(value: string): string {
  if (typeof window === 'undefined') {
    return Buffer.from(value, 'utf-8').toString('base64')
  }
  return window.btoa(value)
}

function guessLanguage(path: string): string {
  const extension = path.split('.').pop()?.toLowerCase()
  switch (extension) {
    case 'ts':
    case 'tsx':
      return 'typescript'
    case 'js':
    case 'jsx':
      return 'javascript'
    case 'json':
      return 'json'
    case 'yml':
    case 'yaml':
      return 'yaml'
    case 'sh':
    case 'bash':
      return 'shell'
    case 'py':
      return 'python'
    case 'go':
      return 'go'
    case 'rs':
      return 'rust'
    case 'rb':
      return 'ruby'
    case 'php':
      return 'php'
    case 'css':
      return 'css'
    case 'html':
    case 'htm':
      return 'html'
    case 'md':
    case 'markdown':
      return 'markdown'
    case 'sql':
      return 'sql'
    default:
      return 'plaintext'
  }
}

export function SftpFileEditor({ sessionId, tabId, path, canWrite }: SftpFileEditorProps) {
  const setTabDirty = useSshWorkspaceStore((state) => state.setTabDirty)
  const [content, setContent] = useState('')
  const [originalContent, setOriginalContent] = useState('')

  const { data, isLoading, error, refetch } = useSftpFileContent(sessionId, path)
  const saveMutation = useSftpSaveFile(sessionId)
  const language = useMemo(() => guessLanguage(path), [path])
  const monacoTheme =
    typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
      ? 'vs-dark'
      : 'vs-light'

  useEffect(() => {
    if (data?.content) {
      const decoded = decodeBase64(data.content)
      setOriginalContent(decoded)
      setContent(decoded)
      setTabDirty(sessionId, tabId, false)
    }
  }, [data?.content, sessionId, setTabDirty, tabId])

  const dirty = useMemo(() => content !== originalContent, [content, originalContent])

  useEffect(() => {
    setTabDirty(sessionId, tabId, dirty)
  }, [dirty, sessionId, setTabDirty, tabId])

  const handleSave = useCallback(async () => {
    if (!dirty || !canWrite) {
      return
    }
    try {
      await saveMutation.mutateAsync({
        path,
        content: encodeBase64(content),
        encoding: 'base64',
        createParents: false,
      })
      setOriginalContent(content)
      setTabDirty(sessionId, tabId, false)
      toast.success('File saved')
    } catch (err) {
      const apiError = toApiError(err)
      toast.error('Unable to save file', { description: apiError.message })
    }
  }, [canWrite, content, dirty, path, saveMutation, sessionId, setTabDirty, tabId])

  const handleRevert = useCallback(() => {
    setContent(originalContent)
  }, [originalContent])

  const handleRefresh = useCallback(() => {
    void refetch()
  }, [refetch])

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
        Loading file…
      </div>
    )
  }

  if (error) {
    const apiError = toApiError(error)
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <p>{apiError.message}</p>
        <Button variant="secondary" size="sm" onClick={handleRefresh}>
          <RefreshCcw className="mr-2 h-4 w-4" aria-hidden />
          Retry
        </Button>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-foreground">{extractNameFromPath(path)}</h3>
          <p className="text-xs text-muted-foreground">{path}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={handleRefresh} disabled={isLoading}>
            <RefreshCcw className="mr-2 h-4 w-4" aria-hidden />
            Refresh
          </Button>
          <Button variant="ghost" size="sm" onClick={handleRevert} disabled={!dirty}>
            <RotateCcw className="mr-2 h-4 w-4" aria-hidden />
            Revert
          </Button>
          <Button
            variant="default"
            size="sm"
            onClick={handleSave}
            disabled={!dirty || !canWrite || saveMutation.isPending}
          >
            {saveMutation.isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
            ) : (
              <Save className="mr-2 h-4 w-4" aria-hidden />
            )}
            Save
          </Button>
        </div>
      </div>

      {!canWrite && (
        <div className="rounded-md border border-dashed border-border/70 bg-muted/40 p-3 text-xs text-muted-foreground">
          You have read-only access. Changes cannot be saved.
        </div>
      )}

      <div
        className="flex-1 overflow-hidden rounded-md border border-border/60 bg-background"
        style={{ minHeight: '320px' }}
      >
        <Editor
          value={content}
          onChange={(value) => setContent(value ?? '')}
          language={language}
          theme={monacoTheme}
          height="100%"
          loading={
            <div className="flex h-full items-center justify-center gap-3 text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
              Initializing editor…
            </div>
          }
          options={{
            readOnly: !canWrite || saveMutation.isPending,
            automaticLayout: true,
            minimap: { enabled: false },
            wordWrap: 'on',
            scrollBeyondLastLine: false,
            fontSize: 14,
            tabSize: 2,
          }}
        />
      </div>
    </div>
  )
}
