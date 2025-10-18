package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database"
)

// SSHProtocolSettings bundles SSH-specific configuration exposed to administrators.
type SSHProtocolSettings struct {
	Session       SessionSettings       `json:"session"`
	Terminal      TerminalSettings      `json:"terminal"`
	Recording     RecordingSettings     `json:"recording"`
	Collaboration CollaborationSettings `json:"collaboration"`
}

// SessionSettings captures default session lifecycle behaviour.
type SessionSettings struct {
	ConcurrentLimit    int  `json:"concurrent_limit"`
	IdleTimeoutMinutes int  `json:"idle_timeout_minutes"`
	EnableSFTP         bool `json:"enable_sftp"`
}

// TerminalSettings describes default terminal appearance.
type TerminalSettings struct {
	ThemeMode  string `json:"theme_mode"`
	FontFamily string `json:"font_family"`
	FontSize   int    `json:"font_size"`
	Scrollback int    `json:"scrollback_limit"`
}

// RecordingSettings captures recorder defaults and retention policy.
type RecordingSettings struct {
	Mode           string `json:"mode"`
	Storage        string `json:"storage"`
	RetentionDays  int    `json:"retention_days"`
	RequireConsent bool   `json:"require_consent"`
}

// CollaborationSettings defines shared-session behaviour defaults.
type CollaborationSettings struct {
	AllowSharing          bool `json:"allow_sharing"`
	RestrictWriteToAdmins bool `json:"restrict_write_to_admins"`
}

// UpdateSSHSettingsInput validates incoming configuration payloads.
type UpdateSSHSettingsInput struct {
	Session       SessionSettingsInput       `json:"session" validate:"required,dive"`
	Terminal      TerminalSettingsInput      `json:"terminal" validate:"required,dive"`
	Recording     RecordingSettingsInput     `json:"recording" validate:"required,dive"`
	Collaboration CollaborationSettingsInput `json:"collaboration" validate:"required,dive"`
}

// SessionSettingsInput validates session defaults provided by administrators.
type SessionSettingsInput struct {
	ConcurrentLimit    int  `json:"concurrent_limit" validate:"min=0,max=1000"`
	IdleTimeoutMinutes int  `json:"idle_timeout_minutes" validate:"min=0,max=10080"`
	EnableSFTP         bool `json:"enable_sftp"`
}

// TerminalSettingsInput validates terminal preference defaults.
type TerminalSettingsInput struct {
	ThemeMode  string `json:"theme_mode" validate:"required,oneof=auto force_dark force_light"`
	FontFamily string `json:"font_family" validate:"required,max=128"`
	FontSize   int    `json:"font_size" validate:"min=8,max=96"`
	Scrollback int    `json:"scrollback_limit" validate:"min=200,max=10000"`
}

// RecordingSettingsInput enumerates mutable recording toggles.
type RecordingSettingsInput struct {
	Mode           string `json:"mode" validate:"required,oneof=disabled optional forced"`
	Storage        string `json:"storage" validate:"required,oneof=filesystem s3"`
	RetentionDays  int    `json:"retention_days" validate:"min=0,max=3650"`
	RequireConsent bool   `json:"require_consent"`
}

// CollaborationSettingsInput validates collaboration defaults.
type CollaborationSettingsInput struct {
	AllowSharing          bool `json:"allow_sharing"`
	RestrictWriteToAdmins bool `json:"restrict_write_to_admins"`
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
		Session:  loadSessionSettings(ctx, s.db),
		Terminal: loadTerminalSettings(ctx, s.db),
		Recording: RecordingSettings{
			Mode:           policy.Mode,
			Storage:        policy.Storage,
			RetentionDays:  policy.RetentionDays,
			RequireConsent: policy.RequireConsent,
		},
		Collaboration: loadCollaborationSettings(ctx, s.db),
	}, nil
}

// UpdateSSHSettings persists the supplied configuration and returns the resulting state.
func (s *ProtocolSettingsService) UpdateSSHSettings(ctx context.Context, actor SessionActor, input UpdateSSHSettingsInput) (SSHProtocolSettings, error) {
	ctx = ensureContext(ctx)

	session := normaliseSessionSettings(input.Session)
	terminal := normaliseTerminalSettings(input.Terminal)
	policy := RecorderPolicy{
		Mode:           input.Recording.Mode,
		Storage:        input.Recording.Storage,
		RetentionDays:  input.Recording.RetentionDays,
		RequireConsent: input.Recording.RequireConsent,
	}
	policy = normalisePolicy(policy)
	collaboration := normaliseCollaborationSettings(input.Collaboration)

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := map[string]string{
			"recording.mode":                           strings.ToLower(strings.TrimSpace(policy.Mode)),
			"recording.storage":                        strings.ToLower(strings.TrimSpace(policy.Storage)),
			"recording.retention_days":                 fmt.Sprintf("%d", policy.RetentionDays),
			"recording.require_consent":                fmt.Sprintf("%t", policy.RequireConsent),
			"sessions.concurrent_limit_default":        fmt.Sprintf("%d", session.ConcurrentLimit),
			"sessions.idle_timeout_minutes":            fmt.Sprintf("%d", session.IdleTimeoutMinutes),
			"protocol.ssh.enable_sftp_default":         fmt.Sprintf("%t", session.EnableSFTP),
			"protocol.ssh.terminal.theme_mode":         terminal.ThemeMode,
			"protocol.ssh.terminal.font_family":        terminal.FontFamily,
			"protocol.ssh.terminal.font_size":          fmt.Sprintf("%d", terminal.FontSize),
			"protocol.ssh.terminal.scrollback_limit":   fmt.Sprintf("%d", terminal.Scrollback),
			"session_sharing.allow_default":            fmt.Sprintf("%t", collaboration.AllowSharing),
			"session_sharing.restrict_write_to_admins": fmt.Sprintf("%t", collaboration.RestrictWriteToAdmins),
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
				"recording_mode":                         policy.Mode,
				"recording_storage":                      policy.Storage,
				"recording_retention_days":               policy.RetentionDays,
				"recording_require_consent":              policy.RequireConsent,
				"sessions_concurrent_limit":              session.ConcurrentLimit,
				"sessions_idle_timeout_minutes":          session.IdleTimeoutMinutes,
				"sessions_enable_sftp":                   session.EnableSFTP,
				"terminal_theme_mode":                    terminal.ThemeMode,
				"terminal_font_family":                   terminal.FontFamily,
				"terminal_font_size":                     terminal.FontSize,
				"terminal_scrollback_limit":              terminal.Scrollback,
				"collaboration_allow_sharing":            collaboration.AllowSharing,
				"collaboration_restrict_write_to_admins": collaboration.RestrictWriteToAdmins,
			},
		}
		auditEntry := buildAuditEntry(actor, entry, actor.UserID)
		_ = s.audit.Log(ctx, auditEntry)
	}

	return s.GetSSHSettings(ctx)
}

func loadSessionSettings(ctx context.Context, db *gorm.DB) SessionSettings {
	settings := SessionSettings{
		ConcurrentLimit:    0,
		IdleTimeoutMinutes: 0,
		EnableSFTP:         true,
	}
	if db == nil {
		return settings
	}

	settings.ConcurrentLimit = parseIntSetting(ctx, db, "sessions.concurrent_limit_default", settings.ConcurrentLimit)
	settings.IdleTimeoutMinutes = parseIntSetting(ctx, db, "sessions.idle_timeout_minutes", settings.IdleTimeoutMinutes)
	settings.EnableSFTP = parseBoolSetting(ctx, db, "protocol.ssh.enable_sftp_default", settings.EnableSFTP)
	return settings
}

func loadTerminalSettings(ctx context.Context, db *gorm.DB) TerminalSettings {
	settings := TerminalSettings{
		ThemeMode:  "auto",
		FontFamily: "monospace",
		FontSize:   14,
		Scrollback: 1000,
	}
	if db == nil {
		return settings
	}

	settings.ThemeMode = normaliseThemeMode(parseStringSetting(ctx, db, "protocol.ssh.terminal.theme_mode", settings.ThemeMode))
	if font := strings.TrimSpace(parseStringSetting(ctx, db, "protocol.ssh.terminal.font_family", settings.FontFamily)); font != "" {
		settings.FontFamily = font
	}
	settings.FontSize = clampInt(parseIntSetting(ctx, db, "protocol.ssh.terminal.font_size", settings.FontSize), 8, 96)
	settings.Scrollback = clampInt(parseIntSetting(ctx, db, "protocol.ssh.terminal.scrollback_limit", settings.Scrollback), 200, 10000)
	return settings
}

func loadCollaborationSettings(ctx context.Context, db *gorm.DB) CollaborationSettings {
	settings := CollaborationSettings{
		AllowSharing:          true,
		RestrictWriteToAdmins: false,
	}
	if db == nil {
		return settings
	}

	settings.AllowSharing = parseBoolSetting(ctx, db, "session_sharing.allow_default", settings.AllowSharing)
	settings.RestrictWriteToAdmins = parseBoolSetting(ctx, db, "session_sharing.restrict_write_to_admins", settings.RestrictWriteToAdmins)
	return settings
}

func normaliseSessionSettings(input SessionSettingsInput) SessionSettings {
	if input.ConcurrentLimit < 0 {
		input.ConcurrentLimit = 0
	}
	if input.IdleTimeoutMinutes < 0 {
		input.IdleTimeoutMinutes = 0
	}
	return SessionSettings{
		ConcurrentLimit:    input.ConcurrentLimit,
		IdleTimeoutMinutes: input.IdleTimeoutMinutes,
		EnableSFTP:         input.EnableSFTP,
	}
}

func normaliseTerminalSettings(input TerminalSettingsInput) TerminalSettings {
	font := strings.TrimSpace(input.FontFamily)
	if font == "" {
		font = "monospace"
	}
	return TerminalSettings{
		ThemeMode:  normaliseThemeMode(input.ThemeMode),
		FontFamily: font,
		FontSize:   clampInt(input.FontSize, 8, 96),
		Scrollback: clampInt(input.Scrollback, 200, 10000),
	}
}

func normaliseCollaborationSettings(input CollaborationSettingsInput) CollaborationSettings {
	return CollaborationSettings{
		AllowSharing:          input.AllowSharing,
		RestrictWriteToAdmins: input.RestrictWriteToAdmins,
	}
}

func normaliseThemeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "force_dark":
		return "force_dark"
	case "force_light":
		return "force_light"
	default:
		return "auto"
	}
}

func parseIntSetting(ctx context.Context, db *gorm.DB, key string, fallback int) int {
	value, err := database.GetSystemSetting(ctx, db, key)
	if err != nil {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
		return parsed
	}
	return fallback
}

func parseBoolSetting(ctx context.Context, db *gorm.DB, key string, fallback bool) bool {
	value, err := database.GetSystemSetting(ctx, db, key)
	if err != nil {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if parsed, parseErr := strconv.ParseBool(value); parseErr == nil {
		return parsed
	}
	return fallback
}

func parseStringSetting(ctx context.Context, db *gorm.DB, key, fallback string) string {
	value, err := database.GetSystemSetting(ctx, db, key)
	if err != nil {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
