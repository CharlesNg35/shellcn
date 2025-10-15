package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database"
)

// SSHProtocolSettings bundles SSH-specific configuration exposed to administrators.
type SSHProtocolSettings struct {
	Recording RecordingSettings `json:"recording"`
}

// RecordingSettings captures recorder defaults and retention policy.
type RecordingSettings struct {
	Mode           string `json:"mode"`
	Storage        string `json:"storage"`
	RetentionDays  int    `json:"retention_days"`
	RequireConsent bool   `json:"require_consent"`
}

// UpdateSSHSettingsInput validates incoming configuration payloads.
type UpdateSSHSettingsInput struct {
	Recording RecordingSettingsInput `json:"recording" validate:"required,dive"`
}

// RecordingSettingsInput enumerates mutable recording toggles.
type RecordingSettingsInput struct {
	Mode           string `json:"mode" validate:"required,oneof=disabled optional forced"`
	Storage        string `json:"storage" validate:"required,oneof=filesystem s3"`
	RetentionDays  int    `json:"retention_days" validate:"min=0,max=3650"`
	RequireConsent bool   `json:"require_consent"`
}

// ProtocolSettingsService coordinates persistence for protocol-level defaults.
type ProtocolSettingsService struct {
	db       *gorm.DB
	audit    *AuditService
	recorder *RecorderService
}

// ProtocolSettingsOption customises ProtocolSettingsService behaviour.
type ProtocolSettingsOption func(*ProtocolSettingsService)

// WithProtocolRecorder attaches the recorder service so policy changes take effect immediately.
func WithProtocolRecorder(recorder *RecorderService) ProtocolSettingsOption {
	return func(svc *ProtocolSettingsService) {
		if recorder != nil {
			svc.recorder = recorder
		}
	}
}

// NewProtocolSettingsService constructs a service once dependencies are supplied.
func NewProtocolSettingsService(db *gorm.DB, audit *AuditService, opts ...ProtocolSettingsOption) (*ProtocolSettingsService, error) {
	if db == nil {
		return nil, fmt.Errorf("protocol settings service: db is required")
	}
	svc := &ProtocolSettingsService{
		db:    db,
		audit: audit,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc, nil
}

// GetSSHSettings returns currently effective SSH defaults.
func (s *ProtocolSettingsService) GetSSHSettings(ctx context.Context) (SSHProtocolSettings, error) {
	ctx = ensureContext(ctx)
	policy := LoadRecorderPolicy(ctx, s.db)
	return SSHProtocolSettings{
		Recording: RecordingSettings{
			Mode:           policy.Mode,
			Storage:        policy.Storage,
			RetentionDays:  policy.RetentionDays,
			RequireConsent: policy.RequireConsent,
		},
	}, nil
}

// UpdateSSHSettings persists the supplied configuration and returns the resulting state.
func (s *ProtocolSettingsService) UpdateSSHSettings(ctx context.Context, actor SessionActor, input UpdateSSHSettingsInput) (SSHProtocolSettings, error) {
	ctx = ensureContext(ctx)

	policy := RecorderPolicy{
		Mode:           input.Recording.Mode,
		Storage:        input.Recording.Storage,
		RetentionDays:  input.Recording.RetentionDays,
		RequireConsent: input.Recording.RequireConsent,
	}
	policy = normalisePolicy(policy)

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := map[string]string{
			"recording.mode":            strings.ToLower(strings.TrimSpace(policy.Mode)),
			"recording.storage":         strings.ToLower(strings.TrimSpace(policy.Storage)),
			"recording.retention_days":  fmt.Sprintf("%d", policy.RetentionDays),
			"recording.require_consent": fmt.Sprintf("%t", policy.RequireConsent),
		}

		for key, value := range settings {
			if err := database.UpsertSystemSetting(ctx, tx, key, value); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return SSHProtocolSettings{}, err
	}

	if s.recorder != nil {
		s.recorder.UpdatePolicy(policy)
	}

	if s.audit != nil {
		entry := AuditEntry{
			Action:   "settings.protocols.ssh_updated",
			Resource: "system:settings.protocols.ssh",
			Result:   "success",
			Metadata: map[string]any{
				"recording_mode":            policy.Mode,
				"recording_storage":         policy.Storage,
				"recording_retention_days":  policy.RetentionDays,
				"recording_require_consent": policy.RequireConsent,
			},
		}
		auditEntry := buildAuditEntry(actor, entry, actor.UserID)
		_ = s.audit.Log(ctx, auditEntry)
	}

	return s.GetSSHSettings(ctx)
}
