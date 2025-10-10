import type { LucideIcon } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/Card'
import type { AuthProviderRecord, AuthProviderType } from '@/types/auth-providers'

interface ProviderCardProps {
  type: AuthProviderType
  name: string
  description: string
  icon: LucideIcon
  provider?: AuthProviderRecord
  onConfigure: () => void
  onToggleEnabled?: (enabled: boolean) => void
  toggleDisabled?: boolean
  toggleLoading?: boolean
  onTestConnection?: () => void
  testDisabled?: boolean
  testLoading?: boolean
}

export function ProviderCard({
  type,
  name,
  description,
  icon: Icon,
  provider,
  onConfigure,
  onToggleEnabled,
  toggleDisabled,
  toggleLoading,
  onTestConnection,
  testDisabled,
  testLoading,
}: ProviderCardProps) {
  const isConfigured = Boolean(provider)
  const isEnabled = provider?.enabled ?? false

  const statusLabel = !isConfigured ? 'Not Configured' : isEnabled ? 'Enabled' : 'Disabled'
  const statusVariant = !isConfigured ? 'outline' : isEnabled ? 'success' : 'secondary'

  const metadata: Array<{ label: string; value: string }> = []

  if (provider) {
    switch (type) {
      case 'local':
        metadata.push(
          {
            label: 'Self-registration',
            value: provider.allowRegistration ? 'Allowed' : 'Disabled',
          },
          {
            label: 'Password reset',
            value: provider.allowPasswordReset ? 'Enabled' : 'Disabled',
          },
          {
            label: 'Email verification',
            value: provider.requireEmailVerification ? 'Required' : 'Optional',
          }
        )
        break
      default:
        metadata.push(
          {
            label: 'Auto-provision',
            value: provider.allowRegistration ? 'Enabled' : 'Disabled',
          },
          {
            label: 'Email verification',
            value: provider.requireEmailVerification ? 'Required' : 'Optional',
          }
        )
    }
  } else {
    metadata.push({
      label: 'Configuration',
      value: 'Provide connection details before enabling this provider',
    })
  }

  const canToggle = Boolean(onToggleEnabled) && isConfigured && !toggleDisabled
  const canTest = Boolean(onTestConnection) && isConfigured && !testDisabled

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-muted">
            <Icon className="h-5 w-5 text-muted-foreground" />
          </div>
          <div>
            <CardTitle className="text-lg">{name}</CardTitle>
            <CardDescription>{description}</CardDescription>
          </div>
        </div>
        <Badge variant={statusVariant}>{statusLabel}</Badge>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="space-y-2 text-sm">
          {metadata.map((item) => (
            <li key={item.label} className="flex items-center justify-between gap-3">
              <span className="font-medium text-foreground">{item.label}</span>
              <span className="text-muted-foreground text-right">{item.value}</span>
            </li>
          ))}
        </ul>
      </CardContent>
      <CardFooter className="flex flex-wrap items-center justify-between gap-2">
        <Button size="sm" onClick={onConfigure}>
          {isConfigured ? 'Edit Configuration' : 'Configure'}
        </Button>
        <div className="flex flex-wrap gap-2">
          {onTestConnection ? (
            <Button
              size="sm"
              variant="outline"
              onClick={onTestConnection}
              disabled={!canTest}
              loading={testLoading}
            >
              Test Connection
            </Button>
          ) : null}
          {onToggleEnabled ? (
            <Button
              size="sm"
              variant={isEnabled ? 'outline' : 'secondary'}
              disabled={!canToggle}
              loading={toggleLoading}
              onClick={() => onToggleEnabled(!isEnabled)}
            >
              {isEnabled ? 'Disable' : 'Enable'}
            </Button>
          ) : null}
        </div>
      </CardFooter>
    </Card>
  )
}
