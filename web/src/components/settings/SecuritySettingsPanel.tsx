import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useShallow } from 'zustand/react/shallow'
import { ShieldCheck, ShieldOff } from 'lucide-react'
import { cn } from '@/lib/utils/cn'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useProfileSettings } from '@/hooks/useProfileSettings'
import {
  passwordChangeSchema,
  totpCodeSchema,
  type PasswordChangeFormValues,
  type TotpCodeFormValues,
} from '@/schemas/profile'
import { useAuthStore } from '@/store/auth-store'
import type { MfaSetupResponse } from '@/types/profile'

interface SecuritySettingsPanelProps {
  className?: string
}

export function SecuritySettingsPanel({ className }: SecuritySettingsPanelProps) {
  const { user } = useAuthStore(useShallow((state) => ({ user: state.user })))
  const { changePassword, setupMfa, enableMfa, disableMfa } = useProfileSettings()
  const [setupResult, setSetupResult] = useState<MfaSetupResponse | null>(null)

  const passwordForm = useForm<PasswordChangeFormValues>({
    resolver: zodResolver(passwordChangeSchema),
    defaultValues: {
      current_password: '',
      new_password: '',
    },
  })

  const enableForm = useForm<TotpCodeFormValues>({
    resolver: zodResolver(totpCodeSchema),
    defaultValues: { code: '' },
  })

  const disableForm = useForm<TotpCodeFormValues>({
    resolver: zodResolver(totpCodeSchema),
    defaultValues: { code: '' },
  })

  const mfaEnabled = Boolean(user?.mfa_enabled || user?.mfa_enrolled)
  const providerLabel = useMemo(() => (user?.auth_provider ?? 'local').toUpperCase(), [user])
  const isLocalProvider = providerLabel === 'LOCAL'

  const handlePasswordSubmit = async (values: PasswordChangeFormValues) => {
    try {
      await changePassword.mutateAsync(values)
      passwordForm.reset()
    } catch {
      // handled via toast
    }
  }

  const beginMfaSetup = async () => {
    try {
      const result = await setupMfa.mutateAsync()
      setSetupResult(result)
      enableForm.reset()
    } catch {
      // toast emitted by mutation
    }
  }

  const handleEnableMfa = async (values: TotpCodeFormValues) => {
    try {
      await enableMfa.mutateAsync({ code: values.code })
      setSetupResult(null)
      enableForm.reset()
    } catch {
      // handled via toast
    }
  }

  const handleDisableMfa = async (values: TotpCodeFormValues) => {
    try {
      await disableMfa.mutateAsync({ code: values.code })
      disableForm.reset()
    } catch {
      // handled via toast
    }
  }

  return (
    <div className={cn('space-y-6', className)}>
      <Card>
        <CardHeader>
          <CardTitle>Password</CardTitle>
          <CardDescription>Update your password to secure access to the platform.</CardDescription>
        </CardHeader>
        <CardContent>
          {!isLocalProvider ? (
            <div className="rounded-md border border-border/60 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
              Passwords are managed by the {providerLabel} identity provider. Update your password
              through the external provider.
            </div>
          ) : (
            <form className="space-y-4" onSubmit={passwordForm.handleSubmit(handlePasswordSubmit)}>
              <Input
                type="password"
                label="Current password"
                placeholder="••••••••"
                {...passwordForm.register('current_password')}
                error={passwordForm.formState.errors.current_password?.message}
                disabled={changePassword.isPending}
              />
              <Input
                type="password"
                label="New password"
                placeholder="Minimum 8 characters"
                {...passwordForm.register('new_password')}
                error={passwordForm.formState.errors.new_password?.message}
                disabled={changePassword.isPending}
              />
              <div className="flex justify-end">
                <Button type="submit" loading={changePassword.isPending}>
                  Update password
                </Button>
              </div>
            </form>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>Multi-factor Authentication</CardTitle>
            <CardDescription>Protect your account with an authenticator app.</CardDescription>
          </div>
          <Badge variant={mfaEnabled ? 'success' : 'outline'} className="flex items-center gap-1">
            {mfaEnabled ? (
              <ShieldCheck className="h-3.5 w-3.5" />
            ) : (
              <ShieldOff className="h-3.5 w-3.5" />
            )}
            {mfaEnabled ? 'Enabled' : 'Disabled'}
          </Badge>
        </CardHeader>
        <CardContent className="space-y-5">
          {!mfaEnabled ? (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Scan the QR code with your authenticator app, then enter the 6-digit verification
                code to enable MFA. Backup codes allow access if you lose the device.
              </p>
              {setupResult ? (
                <div className="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
                  <div className="flex flex-col gap-4 md:flex-row">
                    <img
                      src={`data:image/png;base64,${setupResult.qr_code}`}
                      alt="MFA QR code"
                      className="h-48 w-48 rounded-lg border border-border bg-white p-2"
                    />
                    <div className="space-y-3">
                      <div>
                        <p className="text-sm font-semibold text-foreground">Secret</p>
                        <p className="font-mono text-sm text-muted-foreground break-all">
                          {setupResult.secret}
                        </p>
                      </div>
                      <div>
                        <p className="text-sm font-semibold text-foreground">Backup codes</p>
                        <ul className="grid grid-cols-2 gap-2 text-sm text-muted-foreground">
                          {setupResult.backup_codes.map((code) => (
                            <li key={code} className="rounded-md bg-background px-2 py-1 font-mono">
                              {code}
                            </li>
                          ))}
                        </ul>
                      </div>
                    </div>
                  </div>

                  <form className="space-y-3" onSubmit={enableForm.handleSubmit(handleEnableMfa)}>
                    <Input
                      label="Verification code"
                      placeholder="123456"
                      {...enableForm.register('code')}
                      error={enableForm.formState.errors.code?.message}
                      disabled={enableMfa.isPending}
                    />
                    <div className="flex justify-end gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => setSetupResult(null)}
                        disabled={enableMfa.isPending}
                      >
                        Cancel
                      </Button>
                      <Button type="submit" loading={enableMfa.isPending}>
                        Enable MFA
                      </Button>
                    </div>
                  </form>
                </div>
              ) : (
                <Button
                  type="button"
                  variant="secondary"
                  onClick={beginMfaSetup}
                  loading={setupMfa.isPending}
                >
                  Start MFA setup
                </Button>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                MFA is currently enabled. Enter a code from your authenticator app to disable it if
                you need to revoke access.
              </p>
              <form className="space-y-3" onSubmit={disableForm.handleSubmit(handleDisableMfa)}>
                <Input
                  label="Verification code"
                  placeholder="123456"
                  {...disableForm.register('code')}
                  error={disableForm.formState.errors.code?.message}
                  disabled={disableMfa.isPending}
                />
                <div className="flex justify-end">
                  <Button type="submit" variant="destructive" loading={disableMfa.isPending}>
                    Disable MFA
                  </Button>
                </div>
              </form>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
