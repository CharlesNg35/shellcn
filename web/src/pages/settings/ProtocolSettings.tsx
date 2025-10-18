import { useEffect } from 'react'
import { z } from 'zod'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { Checkbox } from '@/components/ui/Checkbox'
import { useSSHProtocolSettings } from '@/hooks/useProtocolSettings'
import type { SSHProtocolSettings, SSHThemeMode } from '@/types/protocol-settings'
import { toast } from 'sonner'

const sessionSchema = z.object({
  concurrent_limit: z.number().min(0, 'Concurrent limit must be zero or greater').max(1000),
  idle_timeout_minutes: z
    .number()
    .min(0, 'Idle timeout must be zero or greater')
    .max(10080, 'Idle timeout must be less than 10081 minutes'),
  enable_sftp: z.boolean(),
})

const terminalSchema = z.object({
  theme_mode: z.enum(['auto', 'force_dark', 'force_light']),
  font_family: z.string().trim().min(1, 'Font family is required').max(128),
  font_size: z
    .number()
    .min(8, 'Font size must be at least 8')
    .max(96, 'Font size must be at most 96'),
  scrollback_limit: z
    .number()
    .min(200, 'Scrollback must be at least 200 lines')
    .max(10000, 'Scrollback must be at most 10000 lines'),
})

const recordingSchema = z.object({
  mode: z.enum(['disabled', 'optional', 'forced']),
  storage: z.enum(['filesystem', 's3']),
  retention_days: z
    .number()
    .min(0, 'Retention days must be zero or greater')
    .max(3650, 'Retention days must be less than 3651'),
  require_consent: z.boolean(),
})

const collaborationSchema = z.object({
  allow_sharing: z.boolean(),
  restrict_write_to_admins: z.boolean(),
})

const protocolSettingsSchema = z.object({
  session: sessionSchema,
  terminal: terminalSchema,
  recording: recordingSchema,
  collaboration: collaborationSchema,
})

type ProtocolSettingsFormValues = z.infer<typeof protocolSettingsSchema>

const themeOptions: Array<{ value: SSHThemeMode; label: string }> = [
  { value: 'auto', label: 'Match user preference' },
  { value: 'force_dark', label: 'Force dark theme' },
  { value: 'force_light', label: 'Force light theme' },
]

export function ProtocolSettings() {
  const { data, isLoading, isFetching, update } = useSSHProtocolSettings()

  const form = useForm<ProtocolSettingsFormValues>({
    resolver: zodResolver(protocolSettingsSchema),
    defaultValues: {
      session: {
        concurrent_limit: 0,
        idle_timeout_minutes: 0,
        enable_sftp: true,
      },
      terminal: {
        theme_mode: 'auto',
        font_family: 'monospace',
        font_size: 14,
        scrollback_limit: 1000,
      },
      recording: {
        mode: 'optional',
        storage: 'filesystem',
        retention_days: 0,
        require_consent: true,
      },
      collaboration: {
        allow_sharing: true,
        restrict_write_to_admins: false,
      },
    },
  })

  useEffect(() => {
    if (!data) {
      return
    }
    const payload: ProtocolSettingsFormValues = {
      session: {
        concurrent_limit: data.session.concurrent_limit,
        idle_timeout_minutes: data.session.idle_timeout_minutes,
        enable_sftp: data.session.enable_sftp,
      },
      terminal: {
        theme_mode: data.terminal.theme_mode,
        font_family: data.terminal.font_family,
        font_size: data.terminal.font_size,
        scrollback_limit: data.terminal.scrollback_limit,
      },
      recording: {
        mode: data.recording.mode,
        storage: data.recording.storage,
        retention_days: data.recording.retention_days,
        require_consent: data.recording.require_consent,
      },
      collaboration: {
        allow_sharing: data.collaboration.allow_sharing,
        restrict_write_to_admins: data.collaboration.restrict_write_to_admins,
      },
    }
    form.reset(payload)
  }, [data, form])

  const handleSubmit = form.handleSubmit(async (values: ProtocolSettingsFormValues) => {
    const payload: SSHProtocolSettings = {
      session: {
        concurrent_limit: values.session.concurrent_limit,
        idle_timeout_minutes: values.session.idle_timeout_minutes,
        enable_sftp: values.session.enable_sftp,
      },
      terminal: {
        theme_mode: values.terminal.theme_mode,
        font_family: values.terminal.font_family.trim(),
        font_size: values.terminal.font_size,
        scrollback_limit: values.terminal.scrollback_limit,
      },
      recording: {
        mode: values.recording.mode,
        storage: values.recording.storage,
        retention_days: values.recording.retention_days,
        require_consent: values.recording.require_consent,
      },
      collaboration: {
        allow_sharing: values.collaboration.allow_sharing,
        restrict_write_to_admins: values.collaboration.restrict_write_to_admins,
      },
    }

    try {
      await update.mutateAsync(payload)
      toast.success('Protocol settings updated', {
        description: 'SSH defaults saved successfully.',
      })
    } catch (error: unknown) {
      toast.error('Failed to update protocol settings', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    }
  })

  const submitting = update.isPending
  const disabled = isLoading || submitting
  const selectDisabled = disabled || (update.isSuccess && isFetching)

  return (
    <div className="space-y-6">
      <PageHeader
        title="SSH Protocol Settings"
        description="Configure platform-wide defaults for SSH sessions, including concurrency, terminal appearance, recording policy, and collaboration permissions."
      />

      <Card className="p-6 space-y-8">
        <form className="space-y-8" onSubmit={handleSubmit} noValidate>
          <SessionSettingsSection form={form} disabled={disabled} />
          <TerminalSettingsSection
            form={form}
            disabled={disabled}
            selectDisabled={selectDisabled}
          />
          <RecordingSettingsSection
            form={form}
            disabled={disabled}
            selectDisabled={selectDisabled}
          />
          <CollaborationSettingsSection form={form} disabled={disabled} />

          <div className="flex items-center gap-3">
            <Button type="submit" disabled={isLoading || submitting}>
              {submitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              Save changes
            </Button>
            {update.isSuccess && !submitting ? (
              <span className="text-xs text-muted-foreground">Settings saved.</span>
            ) : null}
          </div>
        </form>
      </Card>
    </div>
  )
}

export default ProtocolSettings

interface SectionProps {
  form: UseFormReturn<ProtocolSettingsFormValues>
  disabled: boolean
}

interface TerminalSectionProps extends SectionProps {
  selectDisabled: boolean
}

function SessionSettingsSection({ form, disabled }: SectionProps) {
  const errors = form.formState.errors

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-foreground">Session defaults</h2>
        <p className="text-sm text-muted-foreground">
          Control how many concurrent sessions can be launched and how long inactive sessions remain
          connected by default.
        </p>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="session-concurrent">
            Concurrent session limit
          </label>
          <Input
            id="session-concurrent"
            type="number"
            min={0}
            max={1000}
            value={form.watch('session.concurrent_limit')}
            onChange={(event) =>
              form.setValue('session.concurrent_limit', Number(event.target.value) || 0, {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          <p className="text-xs text-muted-foreground">
            Maximum number of simultaneous SSH sessions per connection. Use 0 for unlimited.
          </p>
          {errors.session?.concurrent_limit ? (
            <p className="text-xs text-rose-500">{errors.session.concurrent_limit.message}</p>
          ) : null}
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="session-idle">
            Idle timeout (minutes)
          </label>
          <Input
            id="session-idle"
            type="number"
            min={0}
            max={10080}
            value={form.watch('session.idle_timeout_minutes')}
            onChange={(event) =>
              form.setValue('session.idle_timeout_minutes', Number(event.target.value) || 0, {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          <p className="text-xs text-muted-foreground">
            Disconnect sessions after this many minutes of inactivity. Use 0 to disable idle
            timeouts.
          </p>
          {errors.session?.idle_timeout_minutes ? (
            <p className="text-xs text-rose-500">{errors.session.idle_timeout_minutes.message}</p>
          ) : null}
        </div>
      </div>

      <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
        <Checkbox
          id="session-enable-sftp"
          checked={form.watch('session.enable_sftp')}
          onCheckedChange={(checked) =>
            form.setValue('session.enable_sftp', Boolean(checked), { shouldDirty: true })
          }
          disabled={disabled}
        />
        <div className="space-y-1">
          <label className="text-sm font-medium text-foreground" htmlFor="session-enable-sftp">
            Enable SFTP by default
          </label>
          <p className="text-xs text-muted-foreground">
            When enabled, newly created SSH connections allow SFTP access unless explicitly
            disabled.
          </p>
        </div>
      </div>
    </section>
  )
}

function TerminalSettingsSection({ form, disabled, selectDisabled }: TerminalSectionProps) {
  const errors = form.formState.errors

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-foreground">Terminal appearance</h2>
        <p className="text-sm text-muted-foreground">
          Define default terminal styling for all users. Individuals can override these values
          through their personal preferences.
        </p>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="terminal-theme">
            Theme mode
          </label>
          <Select
            value={form.watch('terminal.theme_mode')}
            onValueChange={(value) =>
              form.setValue('terminal.theme_mode', value as SSHThemeMode, {
                shouldDirty: true,
              })
            }
            disabled={selectDisabled}
          >
            <SelectTrigger id="terminal-theme">
              <SelectValue placeholder="Select theme behaviour" />
            </SelectTrigger>
            <SelectContent>
              {themeOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="terminal-font">
            Font family
          </label>
          <Input
            id="terminal-font"
            value={form.watch('terminal.font_family')}
            onChange={(event) =>
              form.setValue('terminal.font_family', event.target.value, { shouldDirty: true })
            }
            disabled={disabled}
            placeholder="e.g. Fira Code, JetBrains Mono"
          />
          {errors.terminal?.font_family ? (
            <p className="text-xs text-rose-500">{errors.terminal.font_family.message}</p>
          ) : null}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="terminal-font-size">
            Font size (px)
          </label>
          <Input
            id="terminal-font-size"
            type="number"
            min={8}
            max={96}
            value={form.watch('terminal.font_size')}
            onChange={(event) =>
              form.setValue('terminal.font_size', Number(event.target.value) || 8, {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          {errors.terminal?.font_size ? (
            <p className="text-xs text-rose-500">{errors.terminal.font_size.message}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Applies to newly launched sessions; existing sessions inherit the user’s preference.
            </p>
          )}
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="terminal-scrollback">
            Scrollback limit (lines)
          </label>
          <Input
            id="terminal-scrollback"
            type="number"
            min={200}
            max={10000}
            value={form.watch('terminal.scrollback_limit')}
            onChange={(event) =>
              form.setValue('terminal.scrollback_limit', Number(event.target.value) || 200, {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          {errors.terminal?.scrollback_limit ? (
            <p className="text-xs text-rose-500">{errors.terminal.scrollback_limit.message}</p>
          ) : null}
        </div>
      </div>
    </section>
  )
}

function RecordingSettingsSection({ form, disabled, selectDisabled }: TerminalSectionProps) {
  const errors = form.formState.errors

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-foreground">Session recording</h2>
        <p className="text-sm text-muted-foreground">
          Choose how session recording is applied by default and where captured sessions are stored.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="recording-mode">
            Recording mode
          </label>
          <Select
            value={form.watch('recording.mode')}
            onValueChange={(value) =>
              form.setValue(
                'recording.mode',
                value as ProtocolSettingsFormValues['recording']['mode'],
                {
                  shouldDirty: true,
                }
              )
            }
            disabled={selectDisabled}
          >
            <SelectTrigger id="recording-mode">
              <SelectValue placeholder="Select recording mode" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="disabled">Disabled</SelectItem>
              <SelectItem value="optional">Optional (per-connection opt-in)</SelectItem>
              <SelectItem value="forced">Forced (capture all sessions)</SelectItem>
            </SelectContent>
          </Select>
          {errors.recording?.mode ? (
            <p className="text-xs text-rose-500">{errors.recording.mode.message}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Optional mode only records connections where recording is explicitly enabled. Forced
              mode records every SSH session.
            </p>
          )}
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="recording-storage">
            Recording storage
          </label>
          <Select
            value={form.watch('recording.storage')}
            onValueChange={(value) =>
              form.setValue(
                'recording.storage',
                value as ProtocolSettingsFormValues['recording']['storage'],
                { shouldDirty: true }
              )
            }
            disabled={disabled}
          >
            <SelectTrigger id="recording-storage">
              <SelectValue placeholder="Select storage backend" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="filesystem">Local filesystem</SelectItem>
              <SelectItem value="s3">S3-compatible bucket</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium text-foreground" htmlFor="recording-retention">
            Retention (days)
          </label>
          <Input
            id="recording-retention"
            type="number"
            min={0}
            max={3650}
            value={form.watch('recording.retention_days')}
            onChange={(event) =>
              form.setValue('recording.retention_days', Number(event.target.value) || 0, {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          <p className="text-xs text-muted-foreground">
            Recordings older than this retention window are purged automatically. Use 0 to retain
            indefinitely.
          </p>
          {errors.recording?.retention_days ? (
            <p className="text-xs text-rose-500">{errors.recording.retention_days.message}</p>
          ) : null}
        </div>

        <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
          <Checkbox
            id="recording-consent"
            checked={form.watch('recording.require_consent')}
            onCheckedChange={(checked) =>
              form.setValue('recording.require_consent', Boolean(checked), { shouldDirty: true })
            }
            disabled={disabled}
          />
          <div className="space-y-1">
            <label className="text-sm font-medium text-foreground" htmlFor="recording-consent">
              Require participant consent
            </label>
            <p className="text-xs text-muted-foreground">
              When enabled, participants must acknowledge recording before joining shared sessions.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}

function CollaborationSettingsSection({ form, disabled }: SectionProps) {
  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-foreground">Collaboration</h2>
        <p className="text-sm text-muted-foreground">
          Set collaboration defaults for shared SSH sessions, including whether sharing is permitted
          and who can receive write access.
        </p>
      </div>

      <div className="space-y-3">
        <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
          <Checkbox
            id="collab-allow-sharing"
            checked={form.watch('collaboration.allow_sharing')}
            onCheckedChange={(checked) =>
              form.setValue('collaboration.allow_sharing', Boolean(checked), { shouldDirty: true })
            }
            disabled={disabled}
          />
          <div className="space-y-1">
            <label className="text-sm font-medium text-foreground" htmlFor="collab-allow-sharing">
              Allow sharing by default
            </label>
            <p className="text-xs text-muted-foreground">
              When enabled, new connections start with session sharing turned on. Owners can still
              disable sharing per connection.
            </p>
          </div>
        </div>

        <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
          <Checkbox
            id="collab-restrict-write"
            checked={form.watch('collaboration.restrict_write_to_admins')}
            onCheckedChange={(checked) =>
              form.setValue('collaboration.restrict_write_to_admins', Boolean(checked), {
                shouldDirty: true,
              })
            }
            disabled={disabled}
          />
          <div className="space-y-1">
            <label className="text-sm font-medium text-foreground" htmlFor="collab-restrict-write">
              Restrict write access to admins
            </label>
            <p className="text-xs text-muted-foreground">
              Only administrators can receive write access during shared sessions. Owners keep write
              control regardless of this setting.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
