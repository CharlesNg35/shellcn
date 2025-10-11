import type { ChangeEvent } from 'react'
import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Checkbox } from '@/components/ui/Checkbox'
import { useSettingsPreferences } from '@/hooks/useSettingsPreferences'

interface SessionSettingsPanelProps {
  className?: string
}

export function SessionSettingsPanel({ className }: SessionSettingsPanelProps) {
  const { preferences, updateSessions } = useSettingsPreferences()
  const sessions = preferences.sessions

  const handleNumberChange =
    (field: 'idleTimeoutMinutes' | 'warnBeforeTimeoutMinutes') =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = Number.parseInt(event.target.value, 10)
      if (Number.isNaN(value)) {
        return
      }
      updateSessions({ [field]: Math.max(1, value) })
    }

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Session timeout</CardTitle>
          <CardDescription>Adjust automatic sign-out and idle warnings.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <Input
            type="number"
            label="Automatic logout (minutes)"
            value={sessions.idleTimeoutMinutes}
            min={5}
            onChange={handleNumberChange('idleTimeoutMinutes')}
            helpText="How long to wait before logging out inactive sessions."
          />
          <Input
            type="number"
            label="Warning before logout (minutes)"
            value={sessions.warnBeforeTimeoutMinutes}
            min={1}
            onChange={handleNumberChange('warnBeforeTimeoutMinutes')}
            helpText="Show a reminder before the session expires."
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Trusted devices</CardTitle>
          <CardDescription>Manage remembered devices and reconnection behaviour.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={sessions.rememberDevices}
              onCheckedChange={(checked) => updateSessions({ rememberDevices: Boolean(checked) })}
            />
            Remember this device after MFA
          </label>
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={sessions.autoReconnectSessions}
              onCheckedChange={(checked) =>
                updateSessions({ autoReconnectSessions: Boolean(checked) })
              }
            />
            Automatically reconnect dropped remote sessions
          </label>
        </CardContent>
      </Card>
    </div>
  )
}
