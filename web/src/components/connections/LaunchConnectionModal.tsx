import { useMemo } from 'react'
import { formatDistanceToNow } from 'date-fns'
import { Network, Plug, ShieldAlert, Tags, Users } from 'lucide-react'

import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import type { ConnectionRecord, ConnectionTemplateMetadata } from '@/types/connections'
import type { ConnectionTemplate } from '@/types/protocols'
import type { ActiveConnectionSession } from '@/types/connections'
import type { WorkspaceDescriptor } from '@/workspaces/types'
import type { LaunchSessionOptions } from '@/hooks/useLaunchConnection'

interface LaunchConnectionModalProps {
  open: boolean
  connection: ConnectionRecord | null
  descriptor: WorkspaceDescriptor
  template: ConnectionTemplate | null
  activeSessions: ActiveConnectionSession[]
  isFetchingSessions?: boolean
  isLaunching?: boolean
  errorMessage?: string | null
  onClose: () => void
  onLaunch: (options?: LaunchSessionOptions) => Promise<unknown>
  onResumeSession: (session: ActiveConnectionSession) => void
}

const FEATURE_LABELS: Record<string, string> = {
  supportsSftp: 'SFTP',
  supportsRecording: 'Recording',
  supportsSharing: 'Collaboration',
  supportsSnippets: 'Snippets',
}

function formatTemplateValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '—'
  }
  if (typeof value === 'boolean') {
    return value ? 'Enabled' : 'Disabled'
  }
  if (Array.isArray(value)) {
    return value.map((item) => formatTemplateValue(item)).join(', ')
  }
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return String(value)
    }
  }
  return String(value)
}

function resolveTemplateFields(
  connectionTemplate?: ConnectionTemplateMetadata,
  templateDefinition?: ConnectionTemplate | null
) {
  if (!connectionTemplate?.fields) {
    return []
  }
  const fieldLabels = new Map<string, string>()
  templateDefinition?.sections.forEach((section) => {
    section.fields.forEach((field) => {
      fieldLabels.set(field.key, field.label)
    })
  })
  return Object.entries(connectionTemplate.fields).map(([key, value]) => ({
    key,
    label: fieldLabels.get(key) ?? key,
    value: formatTemplateValue(value),
  }))
}

export function LaunchConnectionModal({
  open,
  connection,
  descriptor,
  template,
  activeSessions,
  isFetchingSessions = false,
  isLaunching = false,
  errorMessage,
  onClose,
  onLaunch,
  onResumeSession,
}: LaunchConnectionModalProps) {
  const templateMetadata = connection?.metadata?.connection_template
  const templateFields = useMemo(
    () => resolveTemplateFields(templateMetadata, template),
    [template, templateMetadata]
  )

  const versionMismatch = useMemo(() => {
    if (!templateMetadata?.version || !template?.version) {
      return false
    }
    return templateMetadata.version !== template.version
  }, [template?.version, templateMetadata?.version])

  if (!connection) {
    return null
  }

  const identityWarning = !connection.identity_id
  const host = connection.targets?.[0]?.host ?? connection.settings?.host
  const port = connection.targets?.[0]?.port ?? connection.settings?.port

  const featureBadges = Object.entries(descriptor.features)
    .filter(([, enabled]) => Boolean(enabled))
    .map(([key]) => FEATURE_LABELS[key] ?? key)

  const handleLaunchClick = async () => {
    try {
      await onLaunch()
    } catch {
      // Errors handled by hook; avoid unhandled rejection
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`Launch ${connection.name}`}
      description="Review connection details, resume active sessions, or start a new workspace."
      size="lg"
    >
      <div className="space-y-5">
        {errorMessage ? (
          <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {errorMessage}
          </div>
        ) : null}

        {identityWarning ? (
          <div className="flex items-start gap-2 rounded-md border border-amber-400/60 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-100">
            <ShieldAlert className="mt-0.5 h-4 w-4" />
            <div>
              <p className="font-medium">Identity required</p>
              <p className="text-xs text-muted-foreground">
                This connection does not have an identity assigned. Launch attempts may fail if the
                protocol requires credentials.
              </p>
            </div>
          </div>
        ) : null}

        {versionMismatch ? (
          <div className="flex items-start gap-2 rounded-md border border-amber-500/60 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-100">
            <ShieldAlert className="mt-0.5 h-4 w-4" />
            <div>
              <p className="font-medium">Template version mismatch</p>
              <p className="text-xs text-muted-foreground">
                The connection template version ({templateMetadata?.version ?? 'unknown'}) differs
                from the latest protocol template ({template?.version ?? 'unknown'}). Update the
                connection to ensure compatibility.
              </p>
            </div>
          </div>
        ) : null}

        <section className="rounded-lg border border-border/60 bg-muted/30 p-4">
          <div className="flex flex-wrap items-center gap-3">
            <Badge variant="outline" className="flex items-center gap-1 text-xs font-semibold">
              <Plug className="h-3.5 w-3.5" />
              {connection.protocol_id.toUpperCase()}
            </Badge>
            {connection.team_id ? (
              <Badge variant="outline" className="flex items-center gap-1 text-xs font-semibold">
                <Users className="h-3.5 w-3.5" />
                Team scoped
              </Badge>
            ) : (
              <Badge variant="secondary" className="flex items-center gap-1 text-xs font-semibold">
                Personal
              </Badge>
            )}
            {featureBadges.map((feature) => (
              <Badge
                key={feature}
                variant="secondary"
                className="text-[10px] uppercase tracking-wide"
              >
                {feature}
              </Badge>
            ))}
          </div>

          <div className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
            <div className="flex items-start gap-2 text-muted-foreground">
              <Network className="mt-0.5 h-4 w-4" />
              <div>
                <p className="text-xs font-medium text-foreground">Host</p>
                <p>{host ?? 'Not specified'}</p>
              </div>
            </div>
            <div className="flex items-start gap-2 text-muted-foreground">
              <Tags className="mt-0.5 h-4 w-4" />
              <div>
                <p className="text-xs font-medium text-foreground">Tags</p>
                <p>
                  {connection.metadata?.tags?.length ? connection.metadata.tags.join(', ') : '—'}
                </p>
              </div>
            </div>
            {port ? (
              <div className="flex items-start gap-2 text-muted-foreground">
                <Network className="mt-0.5 h-4 w-4" />
                <div>
                  <p className="text-xs font-medium text-foreground">Port</p>
                  <p>{port}</p>
                </div>
              </div>
            ) : null}
            {templateMetadata?.driver_id ? (
              <div className="flex items-start gap-2 text-muted-foreground">
                <Plug className="mt-0.5 h-4 w-4" />
                <div>
                  <p className="text-xs font-medium text-foreground">Driver</p>
                  <p>{templateMetadata.driver_id}</p>
                </div>
              </div>
            ) : null}
          </div>
        </section>

        <section>
          <h3 className="text-sm font-semibold text-foreground">Template fields</h3>
          {templateFields.length === 0 ? (
            <p className="mt-2 text-sm text-muted-foreground">No template fields configured.</p>
          ) : (
            <div className="mt-3 space-y-2 text-sm">
              {templateFields.map((field) => (
                <div
                  key={field.key}
                  className="flex items-start justify-between gap-4 rounded-md border border-border/50 bg-background px-3 py-2"
                >
                  <span className="font-medium text-foreground">{field.label}</span>
                  <span className="max-w-[60%] text-right text-muted-foreground">
                    {field.value}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>

        <div className="h-px w-full bg-border" />

        <section>
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-foreground">Active sessions</h3>
            {isFetchingSessions ? (
              <span className="text-xs text-muted-foreground">Checking…</span>
            ) : null}
          </div>
          {activeSessions.length === 0 ? (
            <p className="mt-2 text-sm text-muted-foreground">
              No active sessions. Launch a new workspace to begin.
            </p>
          ) : (
            <div className="mt-3 space-y-2">
              {activeSessions.map((session) => {
                const lastSeenLabel = session.last_seen_at
                  ? formatDistanceToNow(new Date(session.last_seen_at), { addSuffix: true })
                  : 'moments ago'
                return (
                  <div
                    key={session.id}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border/50 bg-background px-3 py-2"
                  >
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-foreground">
                        {session.user_name ?? session.user_id}
                      </p>
                      <p className="text-xs text-muted-foreground">Seen {lastSeenLabel}</p>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => onResumeSession(session)}>
                      Resume
                    </Button>
                  </div>
                )
              })}
            </div>
          )}
        </section>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" onClick={onClose} disabled={isLaunching}>
            Cancel
          </Button>
          <Button onClick={handleLaunchClick} loading={isLaunching} className="sm:min-w-[120px]">
            Launch new session
          </Button>
        </div>
      </div>
    </Modal>
  )
}

export default LaunchConnectionModal
