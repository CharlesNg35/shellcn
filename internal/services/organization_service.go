package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

var (
	// ErrOrganizationNotFound indicates the requested organisation does not exist.
	ErrOrganizationNotFound = errors.New("organization service: organization not found")
)

// CreateOrganizationInput captures the attributes required to register an organisation.
type CreateOrganizationInput struct {
	Name        string
	Description string
	Settings    map[string]any
}

// UpdateOrganizationInput represents mutable organisation fields.
type UpdateOrganizationInput struct {
	Name        *string
	Description *string
	Settings    map[string]any
}

// OrganizationService manages lifecycle operations for organisations.
type OrganizationService struct {
	db           *gorm.DB
	auditService *AuditService
}

// NewOrganizationService constructs an OrganisationService instance.
func NewOrganizationService(db *gorm.DB, auditService *AuditService) (*OrganizationService, error) {
	if db == nil {
		return nil, errors.New("organization service: db is required")
	}
	return &OrganizationService{
		db:           db,
		auditService: auditService,
	}, nil
}

// Create registers a new organisation.
func (s *OrganizationService) Create(ctx context.Context, input CreateOrganizationInput) (*models.Organization, error) {
	ctx = ensureContext(ctx)

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("organization service: name is required")
	}

	org := &models.Organization{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
	}

	if input.Settings != nil {
		data, err := json.Marshal(input.Settings)
		if err != nil {
			return nil, fmt.Errorf("organization service: marshal settings: %w", err)
		}
		org.Settings = datatypes.JSON(data)
	}

	if err := s.db.WithContext(ctx).Create(org).Error; err != nil {
		return nil, fmt.Errorf("organization service: create organization: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "org.create",
		Resource: org.ID,
		Result:   "success",
		Metadata: map[string]any{
			"name": name,
		},
	})

	return org, nil
}

// GetByID loads an organisation and its related entities.
func (s *OrganizationService) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	ctx = ensureContext(ctx)

	var org models.Organization
	err := s.db.WithContext(ctx).
		Preload("Users").
		Preload("Teams").
		First(&org, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrganizationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization service: get organization: %w", err)
	}
	return &org, nil
}

// List returns all organisations ordered by creation date.
func (s *OrganizationService) List(ctx context.Context) ([]models.Organization, error) {
	ctx = ensureContext(ctx)

	var orgs []models.Organization
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&orgs).Error; err != nil {
		return nil, fmt.Errorf("organization service: list organizations: %w", err)
	}
	return orgs, nil
}

// Update modifies metadata for an organisation.
func (s *OrganizationService) Update(ctx context.Context, id string, input UpdateOrganizationInput) (*models.Organization, error) {
	ctx = ensureContext(ctx)

	var org models.Organization
	err := s.db.WithContext(ctx).First(&org, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrganizationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization service: load organization: %w", err)
	}

	updates := map[string]any{}

	if input.Name != nil {
		if name := strings.TrimSpace(*input.Name); name != "" && name != org.Name {
			updates["name"] = name
		}
	}
	if input.Description != nil {
		updates["description"] = strings.TrimSpace(*input.Description)
	}
	if input.Settings != nil {
		data, err := json.Marshal(input.Settings)
		if err != nil {
			return nil, fmt.Errorf("organization service: marshal settings: %w", err)
		}
		updates["settings"] = datatypes.JSON(data)
	}

	if len(updates) == 0 {
		return &org, nil
	}

	if err := s.db.WithContext(ctx).Model(&org).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("organization service: update organization: %w", err)
	}

	if err := s.db.WithContext(ctx).First(&org, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("organization service: reload organization: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "org.update",
		Resource: org.ID,
		Result:   "success",
		Metadata: updates,
	})

	return &org, nil
}

// Delete removes an organisation by identifier.
func (s *OrganizationService) Delete(ctx context.Context, id string) error {
	ctx = ensureContext(ctx)

	var org models.Organization
	err := s.db.WithContext(ctx).First(&org, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrganizationNotFound
	}
	if err != nil {
		return fmt.Errorf("organization service: load organization: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&org).Error; err != nil {
		return fmt.Errorf("organization service: delete organization: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "org.delete",
		Resource: org.ID,
		Result:   "success",
	})

	return nil
}
