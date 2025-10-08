package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

// AuditEntry captures a single audit event to persist.
type AuditEntry struct {
	UserID    *string
	Username  string
	Action    string
	Resource  string
	Result    string
	IPAddress string
	UserAgent string
	Metadata  map[string]any
}

// AuditFilters encapsulates optional filters when querying audit logs.
type AuditFilters struct {
	UserID   string
	Action   string
	Result   string
	Resource string
	Since    *time.Time
	Until    *time.Time
}

// AuditListOptions controls pagination and filtering for audit queries.
type AuditListOptions struct {
	Page     int
	PageSize int
	Filters  AuditFilters
}

// AuditService persists and retrieves audit log entries.
type AuditService struct {
	db *gorm.DB
}

// NewAuditService constructs an AuditService using the provided database handle.
func NewAuditService(db *gorm.DB) (*AuditService, error) {
	if db == nil {
		return nil, errors.New("audit service: db is required")
	}
	return &AuditService{db: db}, nil
}

// Log stores an audit entry, marshalling metadata into JSON form.
func (s *AuditService) Log(ctx context.Context, entry AuditEntry) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(entry.Action) == "" {
		return errors.New("audit service: action is required")
	}
	if strings.TrimSpace(entry.Result) == "" {
		return errors.New("audit service: result is required")
	}

	payload := ""
	if entry.Metadata != nil {
		encoded, err := json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("audit service: marshal metadata: %w", err)
		}
		payload = string(encoded)
	}

	log := models.AuditLog{
		Action:    strings.TrimSpace(entry.Action),
		Resource:  strings.TrimSpace(entry.Resource),
		Result:    strings.TrimSpace(entry.Result),
		Username:  strings.TrimSpace(entry.Username),
		IPAddress: strings.TrimSpace(entry.IPAddress),
		UserAgent: strings.TrimSpace(entry.UserAgent),
		Metadata:  payload,
	}

	if entry.UserID != nil && strings.TrimSpace(*entry.UserID) != "" {
		id := strings.TrimSpace(*entry.UserID)
		log.UserID = &id
	}

	return s.db.WithContext(ctx).Create(&log).Error
}

// List returns paginated audit logs ordered by creation time descending.
func (s *AuditService) List(ctx context.Context, opts AuditListOptions) ([]models.AuditLog, int64, error) {
	ctx = ensureContext(ctx)

	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PageSize
	if perPage <= 0 || perPage > 200 {
		perPage = 50
	}

	var (
		results []models.AuditLog
		total   int64
	)

	query := s.db.WithContext(ctx).Model(&models.AuditLog{})
	query = applyAuditFilters(query, opts.Filters)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("audit service: count logs: %w", err)
	}

	if err := query.
		Preload("User").
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("audit service: list logs: %w", err)
	}

	return results, total, nil
}

// Export returns audit logs that match the provided filters without pagination.
func (s *AuditService) Export(ctx context.Context, filters AuditFilters) ([]models.AuditLog, error) {
	ctx = ensureContext(ctx)

	var logs []models.AuditLog
	query := s.db.WithContext(ctx).Model(&models.AuditLog{})
	query = applyAuditFilters(query, filters)

	if err := query.
		Preload("User").
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("audit service: export logs: %w", err)
	}

	return logs, nil
}

// CleanupOlderThan removes audit logs older than the supplied retention window (in days).
func (s *AuditService) CleanupOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	ctx = ensureContext(ctx)

	if retentionDays <= 0 {
		return 0, errors.New("audit service: retentionDays must be positive")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	if result.Error != nil {
		return 0, fmt.Errorf("audit service: cleanup logs: %w", result.Error)
	}

	return result.RowsAffected, nil
}

func applyAuditFilters(query *gorm.DB, filters AuditFilters) *gorm.DB {
	if filters.UserID != "" {
		query = query.Where("user_id = ?", filters.UserID)
	}
	if filters.Action != "" {
		query = query.Where("action = ?", filters.Action)
	}
	if filters.Result != "" {
		query = query.Where("result = ?", filters.Result)
	}
	if filters.Resource != "" {
		query = query.Where("resource = ?", filters.Resource)
	}
	if filters.Since != nil {
		query = query.Where("created_at >= ?", *filters.Since)
	}
	if filters.Until != nil {
		query = query.Where("created_at <= ?", *filters.Until)
	}
	return query
}
