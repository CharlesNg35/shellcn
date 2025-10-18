import { useEffect } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { useUserPreferences } from '@/hooks/useUserPreferences'
import type { TerminalCursorStyle, UserPreferences } from '@/types/preferences'
import { cn } from '@/lib/utils/cn'
import { toast } from 'sonner'
import { PersonalSnippetsSection } from './PersonalSnippetsSection'

const cursorStyleOptions: Array<{
  value: TerminalCursorStyle
  label: string
  description: string
}> = [
  {
    value: 'block',
    label: 'Block',
    description: 'Classic solid square cursor.',
  },
  {
    value: 'underline',
    label: 'Underline',
    description: 'Low-profile bar under the active cell.',
  },
  {
    value: 'beam',
    label: 'Beam',
    description: 'Thin vertical bar for minimal obstruction.',
  },
]

const preferenceSchema = z.object({
  ssh: z.object({
    terminal: z.object({
      font_family: z.string().trim().min(1, 'Font family is required').max(128),
      cursor_style: z.enum(['block', 'underline', 'beam']),
      copy_on_select: z.boolean(),
      font_size: z
        .number()
        .min(8, 'Font size must be at least 8')
        .max(96, 'Font size must be at most 96'),
      scrollback_limit: z
        .number()
        .min(200, 'Scrollback must be at least 200 lines')
        .max(10000, 'Scrollback must be at most 10000 lines'),
    }),
    sftp: z.object({
      show_hidden_files: z.boolean(),
      auto_open_queue: z.boolean(),
      confirm_before_overwrite: z.boolean(),
    }),
  }),
})

type PreferencesFormValues = z.infer<typeof preferenceSchema>

interface SSHPreferencesPanelProps {
  className?: string
}

export function SSHPreferencesPanel({ className }: SSHPreferencesPanelProps) {
  const { data, isLoading, update } = useUserPreferences()

  const form = useForm<PreferencesFormValues>({
    resolver: zodResolver(preferenceSchema),
    defaultValues: {
      ssh: {
        terminal: {
          font_family: 'Fira Code',
          cursor_style: 'block',
          copy_on_select: true,
          font_size: 14,
          scrollback_limit: 1000,
        },
        sftp: {
          show_hidden_files: false,
          auto_open_queue: true,
          confirm_before_overwrite: true,
        },
      },
    },
  })

  useEffect(() => {
    if (!data) {
      return
    }
    form.reset(data)
  }, [data, form])

  const handleSubmit = form.handleSubmit(async (values: PreferencesFormValues) => {
    const payload: UserPreferences = {
      ssh: {
        terminal: {
          font_family: values.ssh.terminal.font_family.trim(),
          cursor_style: values.ssh.terminal.cursor_style,
          copy_on_select: values.ssh.terminal.copy_on_select,
          font_size: values.ssh.terminal.font_size,
          scrollback_limit: values.ssh.terminal.scrollback_limit,
        },
        sftp: {
          show_hidden_files: values.ssh.sftp.show_hidden_files,
          auto_open_queue: values.ssh.sftp.auto_open_queue,
          confirm_before_overwrite: values.ssh.sftp.confirm_before_overwrite,
        },
      },
    }

    try {
      await update.mutateAsync(payload)
      toast.success('Preferences updated', {
        description: 'Your SSH defaults were saved successfully.',
      })
    } catch (error: unknown) {
      toast.error('Unable to update preferences', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    }
  })

  const submitting = update.isPending

  return (
    <Card className={cn(className)}>
      <CardHeader>
        <CardTitle>SSH Preferences</CardTitle>
        <CardDescription>
          Personalise your SSH terminal and SFTP experience. These preferences override the
          organisation defaults for your account.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-6" onSubmit={handleSubmit} noValidate>
          <div className="space-y-3">
            <h3 className="text-sm font-semibold text-foreground">Terminal</h3>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium text-foreground" htmlFor="pref-terminal-font">
                  Font family
                </label>
                <Input
                  id="pref-terminal-font"
                  value={form.watch('ssh.terminal.font_family')}
                  onChange={(event) =>
                    form.setValue('ssh.terminal.font_family', event.target.value, {
                      shouldDirty: true,
                    })
                  }
                  disabled={isLoading || submitting}
                  placeholder="e.g. Fira Code, JetBrains Mono"
                />
                {form.formState.errors.ssh?.terminal?.font_family ? (
                  <p className="text-xs text-rose-500">
                    {form.formState.errors.ssh.terminal.font_family.message}
                  </p>
                ) : null}
              </div>

              <div className="space-y-2">
                <label
                  className="text-sm font-medium text-foreground"
                  htmlFor="pref-terminal-cursor"
                >
                  Cursor style
                </label>
                <Select
                  value={form.watch('ssh.terminal.cursor_style')}
                  onValueChange={(value) =>
                    form.setValue('ssh.terminal.cursor_style', value as TerminalCursorStyle, {
                      shouldDirty: true,
                    })
                  }
                  disabled={isLoading || submitting}
                >
                  <SelectTrigger id="pref-terminal-cursor">
                    <SelectValue placeholder="Select cursor style" />
                  </SelectTrigger>
                  <SelectContent>
                    {cursorStyleOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Choose how the terminal cursor should appear when you open new sessions.
                </p>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-foreground" htmlFor="pref-terminal-size">
                  Font size (px)
                </label>
                <Input
                  id="pref-terminal-size"
                  type="number"
                  min={8}
                  max={96}
                  value={form.watch('ssh.terminal.font_size')}
                  onChange={(event) =>
                    form.setValue('ssh.terminal.font_size', Number(event.target.value) || 14, {
                      shouldDirty: true,
                    })
                  }
                  disabled={isLoading || submitting}
                />
                {form.formState.errors.ssh?.terminal?.font_size ? (
                  <p className="text-xs text-rose-500">
                    {form.formState.errors.ssh.terminal.font_size.message}
                  </p>
                ) : null}
              </div>

              <div className="space-y-2">
                <label
                  className="text-sm font-medium text-foreground"
                  htmlFor="pref-terminal-scrollback"
                >
                  Scrollback limit
                </label>
                <Input
                  id="pref-terminal-scrollback"
                  type="number"
                  min={200}
                  max={10000}
                  value={form.watch('ssh.terminal.scrollback_limit')}
                  onChange={(event) =>
                    form.setValue(
                      'ssh.terminal.scrollback_limit',
                      Number(event.target.value) || 1000,
                      {
                        shouldDirty: true,
                      }
                    )
                  }
                  disabled={isLoading || submitting}
                />
                {form.formState.errors.ssh?.terminal?.scrollback_limit ? (
                  <p className="text-xs text-rose-500">
                    {form.formState.errors.ssh.terminal.scrollback_limit.message}
                  </p>
                ) : null}
              </div>
            </div>

            <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
              <Checkbox
                id="pref-terminal-copy"
                checked={form.watch('ssh.terminal.copy_on_select')}
                onCheckedChange={(checked) =>
                  form.setValue('ssh.terminal.copy_on_select', Boolean(checked), {
                    shouldDirty: true,
                  })
                }
                disabled={isLoading || submitting}
              />
              <div className="space-y-1">
                <label className="text-sm font-medium text-foreground" htmlFor="pref-terminal-copy">
                  Copy on select
                </label>
                <p className="text-xs text-muted-foreground">
                  Automatically copy highlighted text to your clipboard without pressing a shortcut.
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-3">
            <h3 className="text-sm font-semibold text-foreground">SFTP</h3>
            <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
              <Checkbox
                id="pref-sftp-hidden"
                checked={form.watch('ssh.sftp.show_hidden_files')}
                onCheckedChange={(checked) =>
                  form.setValue('ssh.sftp.show_hidden_files', Boolean(checked), {
                    shouldDirty: true,
                  })
                }
                disabled={isLoading || submitting}
              />
              <div className="space-y-1">
                <label className="text-sm font-medium text-foreground" htmlFor="pref-sftp-hidden">
                  Show hidden files by default
                </label>
                <p className="text-xs text-muted-foreground">
                  Automatically reveal dotfiles (`.*`) when opening the file manager.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
              <Checkbox
                id="pref-sftp-queue"
                checked={form.watch('ssh.sftp.auto_open_queue')}
                onCheckedChange={(checked) =>
                  form.setValue('ssh.sftp.auto_open_queue', Boolean(checked), {
                    shouldDirty: true,
                  })
                }
                disabled={isLoading || submitting}
              />
              <div className="space-y-1">
                <label className="text-sm font-medium text-foreground" htmlFor="pref-sftp-queue">
                  Open transfer queue automatically
                </label>
                <p className="text-xs text-muted-foreground">
                  Keep the transfer queue visible whenever a new upload or download is started.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3 rounded-lg border border-border/70 bg-muted/10 px-3 py-2">
              <Checkbox
                id="pref-sftp-overwrite"
                checked={form.watch('ssh.sftp.confirm_before_overwrite')}
                onCheckedChange={(checked) =>
                  form.setValue('ssh.sftp.confirm_before_overwrite', Boolean(checked), {
                    shouldDirty: true,
                  })
                }
                disabled={isLoading || submitting}
              />
              <div className="space-y-1">
                <label
                  className="text-sm font-medium text-foreground"
                  htmlFor="pref-sftp-overwrite"
                >
                  Confirm before overwriting files
                </label>
                <p className="text-xs text-muted-foreground">
                  When enabled, the file manager asks for confirmation before replacing an existing
                  file.
                </p>
              </div>
            </div>
          </div>

          <PersonalSnippetsSection />

          <div className="flex items-center gap-3">
            <Button type="submit" disabled={isLoading || submitting}>
              {submitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              Save preferences
            </Button>
            {update.isSuccess && !submitting ? (
              <span className="text-xs text-muted-foreground">Preferences saved.</span>
            ) : null}
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
