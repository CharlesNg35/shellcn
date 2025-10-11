import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Checkbox } from '@/components/ui/Checkbox'
import { useSettingsPreferences } from '@/hooks/useSettingsPreferences'

interface NotificationSettingsPanelProps {
  className?: string
}

export function NotificationSettingsPanel({ className }: NotificationSettingsPanelProps) {
  const { preferences, updateNotifications } = useSettingsPreferences()
  const notifications = preferences.notifications

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Email alerts</CardTitle>
          <CardDescription>Choose which events trigger email notifications.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={notifications.emailAlerts}
              onCheckedChange={(checked) => updateNotifications({ emailAlerts: Boolean(checked) })}
            />
            Product updates and release notes
          </label>
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={notifications.securityAlerts}
              onCheckedChange={(checked) =>
                updateNotifications({ securityAlerts: Boolean(checked) })
              }
            />
            Security warnings and policy changes
          </label>
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={notifications.weeklySummary}
              onCheckedChange={(checked) =>
                updateNotifications({ weeklySummary: Boolean(checked) })
              }
            />
            Weekly activity summary
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>In-app notifications</CardTitle>
          <CardDescription>
            Configure desktop and toast alerts while you are online.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={notifications.desktopAlerts}
              onCheckedChange={(checked) =>
                updateNotifications({ desktopAlerts: Boolean(checked) })
              }
            />
            Allow desktop notifications
          </label>
          <label className="flex items-center gap-3 text-sm text-muted-foreground">
            <Checkbox
              checked={notifications.productUpdates}
              onCheckedChange={(checked) =>
                updateNotifications({ productUpdates: Boolean(checked) })
              }
            />
            Show product announcements in-app
          </label>
        </CardContent>
      </Card>
    </div>
  )
}
