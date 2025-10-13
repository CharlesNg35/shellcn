package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/logger"
)

const (
	defaultAuditRetentionDays = 90
	defaultSessionSpec        = "@hourly"
	defaultAuditSpec          = "@daily"
	defaultTokenSpec          = "@daily"
	defaultVaultSpec          = "@weekly"

	jobSessionCleanup = "session_cleanup"
	jobAuditCleanup   = "audit_cleanup"
	jobTokenCleanup   = "token_cleanup"
	jobVaultCleanup   = "vault_cleanup"
)

func recordMaintenance(job string, start time.Time, err error) {
	result := "success"
	message := ""
	if err != nil {
		result = "failure"
		message = err.Error()
	}
	monitoring.RecordMaintenanceRun(job, result, message, time.Since(start))
}

// Cleaner coordinates background maintenance tasks such as purging expired sessions,
// pruning stale audit logs, and removing obsolete tokens.
type Cleaner struct {
	db        *gorm.DB
	sessions  *iauth.SessionService
	audit     *services.AuditService
	cron      *cron.Cron
	now       func() time.Time
	log       *zap.Logger
	enabled   bool
	retention int

	sessionSchedule string
	auditSchedule   string
	tokenSchedule   string
	vaultSchedule   string
	vault           *services.VaultService
}

// Option customises the Cleaner.
type Option func(*Cleaner)

// WithCron injects a preconfigured cron instance, primarily for testing.
func WithCron(c *cron.Cron) Option {
	return func(cleaner *Cleaner) {
		if c != nil {
			cleaner.cron = c
		}
	}
}

// WithNow overrides the clock used for scheduling and cleanup comparisons.
func WithNow(now func() time.Time) Option {
	return func(cleaner *Cleaner) {
		if now != nil {
			cleaner.now = now
		}
	}
}

// WithAuditRetentionDays adjusts how long audit logs are retained before cleanup.
func WithAuditRetentionDays(days int) Option {
	return func(cleaner *Cleaner) {
		if days > 0 {
			cleaner.retention = days
		}
	}
}

// WithSessionSchedule overrides the cron specification for session cleanup.
func WithSessionSchedule(spec string) Option {
	return func(cleaner *Cleaner) {
		if spec != "" {
			cleaner.sessionSchedule = spec
		}
	}
}

// WithAuditSchedule overrides the cron specification for audit retention enforcement.
func WithAuditSchedule(spec string) Option {
	return func(cleaner *Cleaner) {
		if spec != "" {
			cleaner.auditSchedule = spec
		}
	}
}

// WithTokenSchedule overrides the cron specification for token cleanup.
func WithTokenSchedule(spec string) Option {
	return func(cleaner *Cleaner) {
		if spec != "" {
			cleaner.tokenSchedule = spec
		}
	}
}

// WithVaultService wires the vault service for orphan cleanup.
func WithVaultService(vault *services.VaultService, spec ...string) Option {
	return func(cleaner *Cleaner) {
		if vault != nil {
			cleaner.vault = vault
		}
		if len(spec) > 0 && spec[0] != "" {
			cleaner.vaultSchedule = spec[0]
		}
	}
}

// NewCleaner constructs a Cleaner with sensible defaults. Any nil dependency results in
// the corresponding cleanup job being skipped.
func NewCleaner(db *gorm.DB, sessions *iauth.SessionService, audit *services.AuditService, opts ...Option) *Cleaner {
	cleaner := &Cleaner{
		db:              db,
		sessions:        sessions,
		audit:           audit,
		now:             time.Now,
		retention:       defaultAuditRetentionDays,
		sessionSchedule: defaultSessionSpec,
		auditSchedule:   defaultAuditSpec,
		tokenSchedule:   defaultTokenSpec,
		vaultSchedule:   defaultVaultSpec,
		log:             logger.WithModule("maintenance"),
	}

	for _, opt := range opts {
		opt(cleaner)
	}

	if cleaner.cron == nil {
		cleaner.cron = cron.New(cron.WithLogger(cron.DiscardLogger))
	}

	// Determine whether any job is enabled.
	cleaner.enabled = cleaner.sessions != nil || cleaner.audit != nil || cleaner.db != nil || cleaner.vault != nil

	return cleaner
}

// Start registers cleanup jobs with the cron scheduler and launches it if at least one cleanup is enabled.
func (c *Cleaner) Start() error {
	if !c.enabled {
		return nil
	}

	if c.sessions != nil {
		if _, err := c.cron.AddFunc(c.sessionSchedule, func() {
			ctx := context.Background()
			start := time.Now()
			_, runErr := c.sessions.CleanupExpired(ctx)
			recordMaintenance(jobSessionCleanup, start, runErr)
			if runErr != nil {
				c.log.Warn("session cleanup failed", zap.Error(runErr))
			}
		}); err != nil {
			return err
		}
	}

	if c.audit != nil && c.retention > 0 {
		if _, err := c.cron.AddFunc(c.auditSchedule, func() {
			ctx := context.Background()
			start := time.Now()
			_, runErr := c.audit.CleanupOlderThan(ctx, c.retention)
			recordMaintenance(jobAuditCleanup, start, runErr)
			if runErr != nil {
				c.log.Warn("audit cleanup failed", zap.Error(runErr))
			}
		}); err != nil {
			return err
		}
	}

	if c.db != nil {
		if _, err := c.cron.AddFunc(c.tokenSchedule, func() {
			ctx := context.Background()
			start := time.Now()
			_, runErr := CleanupTokens(ctx, c.db, c.now())
			recordMaintenance(jobTokenCleanup, start, runErr)
			if runErr != nil {
				c.log.Warn("token cleanup failed", zap.Error(runErr))
			}
		}); err != nil {
			return err
		}
	}

	if c.vault != nil {
		if _, err := c.cron.AddFunc(c.vaultSchedule, func() {
			ctx := context.Background()
			start := time.Now()
			_, runErr := c.vault.CleanupOrphans(ctx)
			recordMaintenance(jobVaultCleanup, start, runErr)
			if runErr != nil {
				c.log.Warn("vault cleanup failed", zap.Error(runErr))
			}
		}); err != nil {
			return err
		}
	}

	c.cron.Start()
	return nil
}

// Stop halts the underlying scheduler, waiting for any running jobs to complete.
func (c *Cleaner) Stop() context.Context {
	if c.cron == nil {
		return context.Background()
	}
	return c.cron.Stop()
}

// RunOnce executes all configured cleanup routines sequentially. Primarily used in tests
// and during graceful shutdown.
func (c *Cleaner) RunOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var errs error

	if c.sessions != nil {
		start := time.Now()
		_, err := c.sessions.CleanupExpired(ctx)
		recordMaintenance(jobSessionCleanup, start, err)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.audit != nil && c.retention > 0 {
		start := time.Now()
		_, err := c.audit.CleanupOlderThan(ctx, c.retention)
		recordMaintenance(jobAuditCleanup, start, err)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.db != nil {
		start := time.Now()
		_, err := CleanupTokens(ctx, c.db, c.now())
		recordMaintenance(jobTokenCleanup, start, err)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.vault != nil {
		start := time.Now()
		_, err := c.vault.CleanupOrphans(ctx)
		recordMaintenance(jobVaultCleanup, start, err)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}

// TokenCleanupStats captures the number of records removed for each token type.
type TokenCleanupStats struct {
	PasswordResets     int64
	Invites            int64
	EmailVerifications int64
}

// CleanupTokens removes expired or consumed tokens across the core tables.
func CleanupTokens(ctx context.Context, db *gorm.DB, now time.Time) (TokenCleanupStats, error) {
	if db == nil {
		return TokenCleanupStats{}, errors.New("cleanup tokens: db is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	stats := TokenCleanupStats{}

	if result := db.WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&models.PasswordResetToken{}); result.Error != nil {
		return stats, fmt.Errorf("cleanup tokens: password reset tokens: %w", result.Error)
	} else {
		stats.PasswordResets = result.RowsAffected
	}

	if result := db.WithContext(ctx).
		Where("expires_at < ? OR accepted_at IS NOT NULL", now).
		Delete(&models.UserInvite{}); result.Error != nil {
		return stats, fmt.Errorf("cleanup tokens: invites: %w", result.Error)
	} else {
		stats.Invites = result.RowsAffected
	}

	if result := db.WithContext(ctx).
		Where("expires_at < ? OR verified_at IS NOT NULL", now).
		Delete(&models.EmailVerification{}); result.Error != nil {
		return stats, fmt.Errorf("cleanup tokens: email verification: %w", result.Error)
	} else {
		stats.EmailVerifications = result.RowsAffected
	}

	return stats, nil
}
