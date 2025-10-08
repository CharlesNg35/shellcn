import { FileText } from 'lucide-react'

export function AuditLogs() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">Audit Logs</h1>
        <p className="mt-2 text-muted-foreground">View system audit logs and security events</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-12 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <FileText className="h-8 w-8 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-foreground">Audit Log Viewer</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Audit log functionality will be available in Phase 3
        </p>
      </div>
    </div>
  )
}
