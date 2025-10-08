import { Lock } from 'lucide-react'

export function Security() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">Security Settings</h1>
        <p className="mt-2 text-muted-foreground">Configure security policies and MFA settings</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Lock className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">Security Configuration</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Security settings will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
