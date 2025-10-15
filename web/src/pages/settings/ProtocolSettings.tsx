import { useEffect } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
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
import type { RecordingMode, RecordingStorage } from '@/types/protocol-settings'
import { toast } from 'sonner'

const recordingSchema = z.object({
  mode: z.enum(['disabled', 'optional', 'forced']),
  storage: z.enum(['filesystem', 's3']),
  retention_days: z
    .number()
    .min(0, 'Retention days must be zero or greater')
    .max(3650, 'Retention days must be less than 3651'),
  require_consent: z.boolean(),
})

type RecordingFormValues = z.infer<typeof recordingSchema>

export function ProtocolSettings() {
  const { data, isLoading, isFetching, update } = useSSHProtocolSettings()

  const form = useForm<RecordingFormValues>({
    resolver: zodResolver(recordingSchema),
    defaultValues: {
      mode: 'optional',
      storage: 'filesystem',
      retention_days: 0,
      require_consent: true,
    },
  })

  useEffect(() => {
    if (!data) {
      return
    }
    form.reset({
      mode: data.recording.mode,
      storage: data.recording.storage,
      retention_days: data.recording.retention_days,
      require_consent: data.recording.require_consent,
    })
  }, [data, form])

  const handleSubmit = form.handleSubmit(async (values: RecordingFormValues) => {
    try {
      await update.mutateAsync({ recording: values })
      toast.success('Protocol settings updated', {
        description: 'Recording defaults saved successfully.',
      })
    } catch (error: unknown) {
      toast.error('Failed to update recording defaults', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    }
  })

  const submitting = update.isPending

  return (
    <div className="space-y-6">
      <PageHeader
        title="SSH Protocol Settings"
        description="Configure default behaviour for SSH sessions, including recording policies and retention requirements."
      />

      <Card className="p-6">
        <form className="space-y-5" onSubmit={handleSubmit} noValidate>
          <div className="grid gap-2">
            <label className="text-sm font-medium text-foreground" htmlFor="recording-mode">
              Recording mode
            </label>
            <Select
              value={form.watch('mode')}
              onValueChange={(value) =>
                form.setValue('mode', value as RecordingMode, { shouldDirty: true })
              }
              disabled={isLoading || submitting || (update.isSuccess && isFetching)}
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
            {form.formState.errors.mode ? (
              <p className="text-xs text-rose-500">{form.formState.errors.mode.message}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                Optional mode only records sessions that explicitly enable recording. Forced mode
                captures all sessions.
              </p>
            )}
          </div>

          <div className="grid gap-2">
            <label className="text-sm font-medium text-foreground" htmlFor="recording-storage">
              Recording storage
            </label>
            <Select
              value={form.watch('storage')}
              onValueChange={(value) =>
                form.setValue('storage', value as RecordingStorage, { shouldDirty: true })
              }
              disabled={isLoading || submitting}
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

          <div className="grid gap-2">
            <label className="text-sm font-medium text-foreground" htmlFor="retention-days">
              Retention (days)
            </label>
            <Input
              id="retention-days"
              type="number"
              min={0}
              step={1}
              value={form.watch('retention_days')}
              onChange={(event) =>
                form.setValue('retention_days', Number(event.target.value) || 0, {
                  shouldDirty: true,
                })
              }
              disabled={isLoading || submitting}
            />
            <p className="text-xs text-muted-foreground">
              Recordings older than this retention window will be purged automatically. Use 0 to
              retain indefinitely.
            </p>
            {form.formState.errors.retention_days ? (
              <p className="text-xs text-rose-500">
                {form.formState.errors.retention_days.message}
              </p>
            ) : null}
          </div>

          <div className="flex items-start gap-3">
            <Checkbox
              id="require-consent"
              checked={form.watch('require_consent')}
              onCheckedChange={(checked) =>
                form.setValue('require_consent', Boolean(checked), { shouldDirty: true })
              }
              disabled={isLoading || submitting}
            />
            <div>
              <label className="text-sm font-medium text-foreground" htmlFor="require-consent">
                Require participant consent
              </label>
              <p className="text-xs text-muted-foreground">
                When enabled, participants will be prompted to consent before joining a recorded
                session.
              </p>
            </div>
          </div>

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
