package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
)

// CheckStatus captures the outcome of a security audit check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// Check contains the result of a single audit verification.
type Check struct {
	ID          string      `json:"id"`
	Status      CheckStatus `json:"status"`
	Message     string      `json:"message"`
	Remediation string      `json:"remediation,omitempty"`
	Details     any         `json:"details,omitempty"`
}

// Result aggregates all checks with a simple status summary.
type Result struct {
	CheckedAt time.Time      `json:"checked_at"`
	Checks    []Check        `json:"checks"`
	Summary   map[string]int `json:"summary"`
}

// AuditService evaluates core security controls and configuration.
type AuditService struct {
	db  *gorm.DB
	jwt *iauth.JWTService
	cfg *app.Config
	now func() time.Time
}

// NewAuditService constructs the audit service. All dependencies are optional; missing
// inputs degrade specific checks to warnings.
func NewAuditService(db *gorm.DB, jwt *iauth.JWTService, cfg *app.Config) *AuditService {
	return &AuditService{
		db:  db,
		jwt: jwt,
		cfg: cfg,
		now: time.Now,
	}
}

// WithClock overrides the clock used in results (primarily for testing).
func (s *AuditService) WithClock(clock func() time.Time) {
	if clock != nil {
		s.now = clock
	}
}

// Run executes all audit checks and returns their outcome.
func (s *AuditService) Run(ctx context.Context) Result {
	if ctx == nil {
		ctx = context.Background()
	}

	checks := []Check{
		s.checkRootUser(ctx),
		s.checkJWTSecret(),
		s.checkVaultKey(),
		s.checkSessionTTL(),
	}

	summary := map[string]int{
		string(StatusPass): 0,
		string(StatusWarn): 0,
		string(StatusFail): 0,
	}

	for _, check := range checks {
		summary[string(check.Status)]++
	}

	return Result{
		CheckedAt: s.now().UTC(),
		Checks:    checks,
		Summary:   summary,
	}
}

func (s *AuditService) checkRootUser(ctx context.Context) Check {
	if s.db == nil {
		return Check{
			ID:          "root_user_present",
			Status:      StatusWarn,
			Message:     "Database unavailable – unable to confirm root user presence",
			Remediation: "Ensure database connectivity before running the audit.",
		}
	}

	var count int64
	if err := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("is_root = ?", true).
		Count(&count).Error; err != nil {
		return Check{
			ID:          "root_user_present",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("Could not verify root users: %v", err),
			Remediation: "Retry after resolving database errors.",
		}
	}

	if count == 0 {
		return Check{
			ID:          "root_user_present",
			Status:      StatusFail,
			Message:     "No active root user found.",
			Remediation: "Create an active root user to guarantee emergency access.",
		}
	}

	return Check{
		ID:      "root_user_present",
		Status:  StatusPass,
		Message: "Root user present.",
		Details: map[string]any{"count": count},
	}
}

func (s *AuditService) checkJWTSecret() Check {
	if s.jwt == nil {
		return Check{
			ID:          "jwt_secret_strength",
			Status:      StatusWarn,
			Message:     "JWT service not initialised – unable to assess signing secret strength.",
			Remediation: "Initialise JWT service with a strong secret.",
		}
	}

	length := s.jwt.SecretLength()

	switch {
	case length == 0:
		return Check{
			ID:          "jwt_secret_strength",
			Status:      StatusFail,
			Message:     "Missing JWT signing secret.",
			Remediation: "Provide a cryptographically secure signing secret (>= 32 bytes).",
		}
	case length < 32:
		return Check{
			ID:          "jwt_secret_strength",
			Status:      StatusFail,
			Message:     fmt.Sprintf("JWT signing secret is too short (%d bytes).", length),
			Remediation: "Use a randomly generated secret of at least 32 bytes.",
		}
	case length < 48:
		return Check{
			ID:          "jwt_secret_strength",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("JWT signing secret is %d bytes. Consider increasing to 48+ bytes.", length),
			Remediation: "Increase the length of SHELLCN_AUTH_JWT_SECRET to at least 48 bytes.",
			Details:     map[string]any{"length": length},
		}
	default:
		return Check{
			ID:      "jwt_secret_strength",
			Status:  StatusPass,
			Message: fmt.Sprintf("JWT signing secret length is %d bytes.", length),
			Details: map[string]any{"length": length},
		}
	}
}

func (s *AuditService) checkVaultKey() Check {
	if s.cfg == nil {
		return Check{
			ID:          "vault_encryption_key",
			Status:      StatusWarn,
			Message:     "Configuration not loaded – unable to verify vault encryption key.",
			Remediation: "Load configuration before running the security audit.",
		}
	}

	key := strings.TrimSpace(s.cfg.Vault.EncryptionKey)
	if key == "" {
		return Check{
			ID:          "vault_encryption_key",
			Status:      StatusFail,
			Message:     "Vault encryption key is not configured.",
			Remediation: "Set SHELLCN_VAULT_ENCRYPTION_KEY to a 32+ byte random value.",
		}
	}

	length := len(key)
	if length < 32 {
		return Check{
			ID:          "vault_encryption_key",
			Status:      StatusFail,
			Message:     fmt.Sprintf("Vault encryption key is too short (%d characters).", length),
			Remediation: "Use an encryption key of at least 32 characters for AES-256-GCM.",
		}
	}

	return Check{
		ID:      "vault_encryption_key",
		Status:  StatusPass,
		Message: "Vault encryption key configured.",
		Details: map[string]any{"length": length},
	}
}

func (s *AuditService) checkSessionTTL() Check {
	if s.cfg == nil {
		return Check{
			ID:          "session_refresh_ttl",
			Status:      StatusWarn,
			Message:     "Configuration not loaded – unable to evaluate session lifetime.",
			Remediation: "Load configuration before running the security audit.",
		}
	}

	ttl := s.cfg.Auth.Session.RefreshTTL
	if ttl <= 0 {
		return Check{
			ID:          "session_refresh_ttl",
			Status:      StatusWarn,
			Message:     "Refresh token TTL is not configured; using default duration.",
			Remediation: "Set SHELLCN_AUTH_SESSION_REFRESH_TOKEN_TTL to control session lifetime.",
		}
	}

	const maxRecommended = 30 * 24 * time.Hour

	if ttl > maxRecommended {
		return Check{
			ID:          "session_refresh_ttl",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("Refresh token TTL (%s) exceeds recommended maximum (%s).", ttl, maxRecommended),
			Remediation: "Reduce refresh token TTL to 30 days or lower to limit credential exposure.",
			Details:     map[string]any{"ttl": ttl.String()},
		}
	}

	return Check{
		ID:      "session_refresh_ttl",
		Status:  StatusPass,
		Message: fmt.Sprintf("Refresh token TTL is %s.", ttl),
		Details: map[string]any{"ttl": ttl.String()},
	}
}
