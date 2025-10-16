package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// UserPreferences represents persisted user-level customisations.
type UserPreferences struct {
	SSH SSHPreferences `json:"ssh"`
}

// SSHPreferences groups terminal and SFTP preferences.
type SSHPreferences struct {
	Terminal SSHTerminalPreferences `json:"terminal"`
	SFTP     SSHSFTPPreferences     `json:"sftp"`
}

// SSHTerminalPreferences describes per-user terminal appearance and behaviour controls.
type SSHTerminalPreferences struct {
	FontFamily   string `json:"font_family"`
	CursorStyle  string `json:"cursor_style"`
	CopyOnSelect bool   `json:"copy_on_select"`
}

// SSHSFTPPreferences enumerates user-tunable SFTP behaviour.
type SSHSFTPPreferences struct {
	ShowHiddenFiles bool `json:"show_hidden_files"`
	AutoOpenQueue   bool `json:"auto_open_queue"`
}

// UserPreferencesService coordinates CRUD operations for user preference data.
type UserPreferencesService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewUserPreferencesService constructs a UserPreferencesService with the supplied dependencies.
func NewUserPreferencesService(db *gorm.DB, audit *AuditService) (*UserPreferencesService, error) {
	if db == nil {
		return nil, fmt.Errorf("user preferences service: db is required")
	}
	return &UserPreferencesService{
		db:    db,
		audit: audit,
	}, nil
}

// Get returns the effective preference set for the specified user.
func (s *UserPreferencesService) Get(ctx context.Context, userID string) (UserPreferences, error) {
	ctx = ensureContext(ctx)
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DefaultUserPreferences(), apperrors.NewBadRequest("user id is required")
	}

	var user struct {
		ID          string
		Preferences datatypes.JSONMap
	}

	err := s.db.WithContext(ctx).
		Table("users").
		Select("id", "preferences").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultUserPreferences(), ErrUserNotFound
		}
		return DefaultUserPreferences(), fmt.Errorf("user preferences service: load user preferences: %w", err)
	}

	return NormaliseUserPreferences(user.Preferences), nil
}

// Update persists preference changes for the specified user.
func (s *UserPreferencesService) Update(ctx context.Context, userID string, prefs UserPreferences) (UserPreferences, error) {
	ctx = ensureContext(ctx)
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DefaultUserPreferences(), apperrors.NewBadRequest("user id is required")
	}

	var user struct {
		ID          string
		Preferences datatypes.JSONMap
	}

	err := s.db.WithContext(ctx).
		Table("users").
		Select("id", "preferences").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultUserPreferences(), ErrUserNotFound
		}
		return DefaultUserPreferences(), fmt.Errorf("user preferences service: load user: %w", err)
	}

	sanitised := sanitizeUserPreferences(prefs)
	payload := MarshalUserPreferences(sanitised)

	err = s.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		Update("preferences", payload).Error
	if err != nil {
		return DefaultUserPreferences(), fmt.Errorf("user preferences service: update preferences: %w", err)
	}

	err = s.db.WithContext(ctx).
		Table("users").
		Select("preferences").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return DefaultUserPreferences(), fmt.Errorf("user preferences service: reload preferences: %w", err)
	}

	result := NormaliseUserPreferences(user.Preferences)

	if s.audit != nil {
		entry := AuditEntry{
			Action:   "user.preferences.update",
			Resource: userID,
			Result:   "success",
			Metadata: map[string]any{
				"ssh_terminal_font_family":    result.SSH.Terminal.FontFamily,
				"ssh_terminal_cursor_style":   result.SSH.Terminal.CursorStyle,
				"ssh_terminal_copy_on_select": result.SSH.Terminal.CopyOnSelect,
				"ssh_sftp_show_hidden":        result.SSH.SFTP.ShowHiddenFiles,
				"ssh_sftp_auto_open_queue":    result.SSH.SFTP.AutoOpenQueue,
			},
		}
		_ = s.audit.Log(ctx, entry)
	}

	return result, nil
}

// DefaultUserPreferences returns the canonical defaults applied when no user preference exists.
func DefaultUserPreferences() UserPreferences {
	return UserPreferences{
		SSH: SSHPreferences{
			Terminal: SSHTerminalPreferences{
				FontFamily:   "Fira Code",
				CursorStyle:  "block",
				CopyOnSelect: true,
			},
			SFTP: SSHSFTPPreferences{
				ShowHiddenFiles: false,
				AutoOpenQueue:   true,
			},
		},
	}
}

// NormaliseUserPreferences coerces the raw JSON map (if any) into a strongly typed structure with defaults applied.
func NormaliseUserPreferences(raw datatypes.JSONMap) UserPreferences {
	prefs := DefaultUserPreferences()
	if len(raw) == 0 {
		return prefs
	}

	sshNode, ok := toMap(raw["ssh"])
	if !ok {
		return prefs
	}

	if terminalNode, ok := toMap(sshNode["terminal"]); ok {
		if font, ok := asString(terminalNode["font_family"]); ok && strings.TrimSpace(font) != "" {
			prefs.SSH.Terminal.FontFamily = strings.TrimSpace(font)
		}
		if cursor, ok := asString(terminalNode["cursor_style"]); ok {
			prefs.SSH.Terminal.CursorStyle = normaliseCursorStyle(cursor)
		}
		if copyOnSelect, ok := asBool(terminalNode["copy_on_select"]); ok {
			prefs.SSH.Terminal.CopyOnSelect = copyOnSelect
		}
	}

	if sftpNode, ok := toMap(sshNode["sftp"]); ok {
		if showHidden, ok := asBool(sftpNode["show_hidden_files"]); ok {
			prefs.SSH.SFTP.ShowHiddenFiles = showHidden
		}
		if autoQueue, ok := asBool(sftpNode["auto_open_queue"]); ok {
			prefs.SSH.SFTP.AutoOpenQueue = autoQueue
		}
	}

	return prefs
}

// MarshalUserPreferences converts the structured preferences into the JSON map persisted in the database.
func MarshalUserPreferences(prefs UserPreferences) datatypes.JSONMap {
	terminal := map[string]any{
		"font_family":    strings.TrimSpace(prefs.SSH.Terminal.FontFamily),
		"cursor_style":   normaliseCursorStyle(prefs.SSH.Terminal.CursorStyle),
		"copy_on_select": prefs.SSH.Terminal.CopyOnSelect,
	}

	sftp := map[string]any{
		"show_hidden_files": prefs.SSH.SFTP.ShowHiddenFiles,
		"auto_open_queue":   prefs.SSH.SFTP.AutoOpenQueue,
	}

	return datatypes.JSONMap{
		"ssh": map[string]any{
			"terminal": terminal,
			"sftp":     sftp,
		},
	}
}

func sanitizeUserPreferences(input UserPreferences) UserPreferences {
	defaults := DefaultUserPreferences()

	if trimmed := strings.TrimSpace(input.SSH.Terminal.FontFamily); trimmed != "" {
		defaults.SSH.Terminal.FontFamily = trimmed
	}
	defaults.SSH.Terminal.CursorStyle = normaliseCursorStyle(input.SSH.Terminal.CursorStyle)
	defaults.SSH.Terminal.CopyOnSelect = input.SSH.Terminal.CopyOnSelect
	defaults.SSH.SFTP.ShowHiddenFiles = input.SSH.SFTP.ShowHiddenFiles
	defaults.SSH.SFTP.AutoOpenQueue = input.SSH.SFTP.AutoOpenQueue

	return defaults
}

func normaliseCursorStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "block":
		return "block"
	case "underline":
		return "underline"
	case "bar", "beam", "line", "vertical":
		return "beam"
	default:
		return "block"
	}
}

func toMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case datatypes.JSONMap:
		return map[string]any(typed), true
	default:
		return nil, false
	}
}

func asString(value any) (string, bool) {
	str, ok := value.(string)
	if ok {
		return str, true
	}
	return "", false
}

func asBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		if strings.TrimSpace(v) == "" {
			return false, false
		}
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, false
		}
		return parsed, true
	default:
		return false, false
	}
}
