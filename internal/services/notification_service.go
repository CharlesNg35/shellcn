package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/realtime"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// NotificationDTO represents the API-friendly notification payload.
type NotificationDTO struct {
	ID        string               `json:"id"`
	UserID    string               `json:"user_id"`
	Type      string               `json:"type"`
	Title     string               `json:"title"`
	Message   string               `json:"message"`
	Severity  string               `json:"severity"`
	ActionURL string               `json:"action_url,omitempty"`
	Metadata  map[string]any       `json:"metadata,omitempty"`
	IsRead    bool                 `json:"is_read"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	ReadAt    *time.Time           `json:"read_at,omitempty"`
	Raw       *models.Notification `json:"-"`
}

// CreateNotificationInput defines attributes required to persist a notification.
type CreateNotificationInput struct {
	UserID    string
	Type      string
	Title     string
	Message   string
	Severity  string
	ActionURL string
	Metadata  map[string]any
	IsRead    bool
}

// ListNotificationsInput defines filters for querying user notifications.
type ListNotificationsInput struct {
	UserID string
	Limit  int
	Offset int
}

// NotificationEventPayload represents data sent to realtime consumers.
type NotificationEventPayload struct {
	Notification   *NotificationDTO `json:"notification,omitempty"`
	NotificationID string           `json:"notification_id,omitempty"`
}

// NotificationService manages user in-app notifications.
type NotificationService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(db *gorm.DB, hub *realtime.Hub) (*NotificationService, error) {
	if db == nil {
		return nil, errors.New("notification service: db is required")
	}
	return &NotificationService{db: db, hub: hub}, nil
}

// ListForUser returns notifications for the supplied user ordered by recency.
func (s *NotificationService) ListForUser(ctx context.Context, input ListNotificationsInput) ([]NotificationDTO, error) {
	ctx = ensureContext(ctx)
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("notification service: user id is required")
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	var rows []models.Notification
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(max(0, input.Offset)).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("notification service: list notifications: %w", err)
	}

	return mapNotificationRows(rows), nil
}

// Create registers a new notification and optionally broadcasts the event.
func (s *NotificationService) Create(ctx context.Context, input CreateNotificationInput) (*NotificationDTO, error) {
	ctx = ensureContext(ctx)
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("notification service: user id is required")
	}
	notificationType := strings.TrimSpace(input.Type)
	if notificationType == "" {
		return nil, errors.New("notification service: type is required")
	}

	notification := models.Notification{
		UserID:    userID,
		Type:      notificationType,
		Title:     strings.TrimSpace(input.Title),
		Message:   strings.TrimSpace(input.Message),
		Severity:  strings.TrimSpace(defaultIfEmpty(input.Severity, "info")),
		ActionURL: strings.TrimSpace(input.ActionURL),
		IsRead:    input.IsRead,
	}

	if input.Metadata != nil {
		if data, err := json.Marshal(input.Metadata); err == nil {
			notification.Metadata = datatypes.JSON(data)
		} else {
			return nil, fmt.Errorf("notification service: marshal metadata: %w", err)
		}
	}

	if input.IsRead {
		now := time.Now().UTC()
		notification.ReadAt = &now
	}

	if err := s.db.WithContext(ctx).Create(&notification).Error; err != nil {
		return nil, fmt.Errorf("notification service: create notification: %w", err)
	}

	dto := mapNotification(notification)
	s.broadcast(userID, "notification.created", &NotificationEventPayload{
		Notification: &dto,
	})
	return &dto, nil
}

// MarkRead sets the notification read flag for a user.
func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID string) (*NotificationDTO, error) {
	ctx = ensureContext(ctx)
	var notification models.Notification
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		First(&notification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("notification service: load notification: %w", err)
	}

	now := time.Now().UTC()
	notification.IsRead = true
	notification.ReadAt = &now

	if err := s.db.WithContext(ctx).Model(&notification).
		Updates(map[string]any{
			"is_read": true,
			"read_at": now,
		}).Error; err != nil {
		return nil, fmt.Errorf("notification service: mark read: %w", err)
	}

	dto := mapNotification(notification)
	dto.IsRead = true
	dto.ReadAt = &now

	s.broadcast(userID, "notification.read", &NotificationEventPayload{
		Notification:   &dto,
		NotificationID: notification.ID,
	})

	return &dto, nil
}

// MarkUnread unsets the notification read flag.
func (s *NotificationService) MarkUnread(ctx context.Context, userID, notificationID string) (*NotificationDTO, error) {
	ctx = ensureContext(ctx)
	var notification models.Notification
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		First(&notification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("notification service: load notification: %w", err)
	}

	if err := s.db.WithContext(ctx).Model(&notification).
		Updates(map[string]any{
			"is_read": false,
			"read_at": nil,
		}).Error; err != nil {
		return nil, fmt.Errorf("notification service: mark unread: %w", err)
	}

	notification.IsRead = false
	notification.ReadAt = nil
	dto := mapNotification(notification)

	s.broadcast(userID, "notification.updated", &NotificationEventPayload{
		Notification:   &dto,
		NotificationID: notification.ID,
	})

	return &dto, nil
}

// Delete removes a notification owned by the supplied user.
func (s *NotificationService) Delete(ctx context.Context, userID, notificationID string) error {
	ctx = ensureContext(ctx)
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Delete(&models.Notification{})
	if result.Error != nil {
		return fmt.Errorf("notification service: delete notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}

	s.broadcast(userID, "notification.deleted", &NotificationEventPayload{
		NotificationID: notificationID,
	})
	return nil
}

// MarkAllRead marks all notifications for the user as read.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	ctx = ensureContext(ctx)
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]any{
			"is_read": true,
			"read_at": now,
		}).Error; err != nil {
		return fmt.Errorf("notification service: mark all read: %w", err)
	}

	s.broadcast(userID, "notification.read_all", nil)
	return nil
}

func (s *NotificationService) broadcast(userID, event string, payload *NotificationEventPayload) {
	if s.hub == nil {
		return
	}
	message := realtime.Message{
		Stream: realtime.StreamNotifications,
		Event:  event,
	}
	if payload != nil {
		message.Data = payload
	}
	s.hub.BroadcastToUser(realtime.StreamNotifications, userID, message)
}

func mapNotificationRows(rows []models.Notification) []NotificationDTO {
	items := make([]NotificationDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapNotification(row))
	}
	return items
}

func mapNotification(row models.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        row.ID,
		UserID:    row.UserID,
		Type:      row.Type,
		Title:     row.Title,
		Message:   row.Message,
		Severity:  defaultIfEmpty(row.Severity, "info"),
		ActionURL: row.ActionURL,
		Metadata:  decodeJSON(row.Metadata),
		IsRead:    row.IsRead,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		ReadAt:    row.ReadAt,
		Raw:       &row,
	}
}

func decodeJSON(data datatypes.JSON) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
