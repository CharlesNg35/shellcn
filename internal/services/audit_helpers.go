package services

import "context"

// recordAudit logs the supplied entry while tolerating audit failures.
func recordAudit(audit *AuditService, ctx context.Context, entry AuditEntry) {
	if audit == nil {
		return
	}
	_ = audit.Log(ctx, entry)
}
