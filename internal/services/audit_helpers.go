package services

import (
	"context"
	"strings"

	"github.com/charlesng35/shellcn/internal/auditctx"
)

// recordAudit logs the supplied entry while tolerating audit failures.
func recordAudit(audit *AuditService, ctx context.Context, entry AuditEntry) {
	if audit == nil {
		return
	}

	if actor, ok := auditctx.FromContext(ctx); ok {
		if entry.UserID == nil && strings.TrimSpace(actor.UserID) != "" {
			id := strings.TrimSpace(actor.UserID)
			entry.UserID = &id
		}
		if entry.Username == "" && strings.TrimSpace(actor.Username) != "" {
			entry.Username = strings.TrimSpace(actor.Username)
		}
		if entry.IPAddress == "" && actor.IPAddress != "" {
			entry.IPAddress = actor.IPAddress
		}
		if entry.UserAgent == "" && actor.UserAgent != "" {
			entry.UserAgent = actor.UserAgent
		}
	}

	if entry.Username == "" && entry.UserID != nil {
		entry.Username = *entry.UserID
	}

	_ = audit.Log(ctx, entry)
}
