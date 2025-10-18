package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

var (
	// ErrSnippetNotFound indicates the requested snippet does not exist.
	ErrSnippetNotFound = errors.New("snippet service: snippet not found")
)

// SnippetService manages CRUD operations for command snippets.
type SnippetService struct {
	db *gorm.DB
}

// NewSnippetService constructs a snippet service once a database handle is supplied.
func NewSnippetService(db *gorm.DB) (*SnippetService, error) {
	if db == nil {
		return nil, errors.New("snippet service: db is required")
	}
	return &SnippetService{db: db}, nil
}

func ensuredContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// ListSnippetsOptions controls how snippets are filtered.
type ListSnippetsOptions struct {
	Scope             string
	ConnectionID      string
	OwnerUserID       string
	IncludeGlobal     bool
	IncludeConnection bool
	IncludeUser       bool
}

// CreateSnippetInput captures required fields when creating a snippet.
type CreateSnippetInput struct {
	Name         string
	Description  string
	Command      string
	Scope        string
	OwnerUserID  string
	ConnectionID string
}

// UpdateSnippetInput describes mutable snippet fields. A nil pointer indicates no change.
type UpdateSnippetInput struct {
	Name         *string
	Description  *string
	Command      *string
	Scope        *string
	ConnectionID *string
	OwnerUserID  string
}

// List retrieves snippets matching the supplied options.
func (s *SnippetService) List(ctx context.Context, opts ListSnippetsOptions) ([]models.Snippet, error) {
	if s == nil {
		return nil, errors.New("snippet service: service not initialised")
	}
	ctx = ensuredContext(ctx)

	scope := normalizeScope(opts.Scope)
	connectionID := strings.TrimSpace(opts.ConnectionID)
	ownerID := strings.TrimSpace(opts.OwnerUserID)

	dedupe := make(map[string]struct{})
	var snippets []models.Snippet

	appendRecords := func(records []models.Snippet) {
		for _, record := range records {
			if _, exists := dedupe[record.ID]; exists {
				continue
			}
			dedupe[record.ID] = struct{}{}
			snippets = append(snippets, record)
		}
	}

	queryByScope := func(scope string, builder func(*gorm.DB) *gorm.DB) error {
		var rows []models.Snippet
		q := s.db.WithContext(ctx).Model(&models.Snippet{}).Where("scope = ?", scope)
		if builder != nil {
			q = builder(q)
		}
		if err := q.Order("LOWER(name)").Find(&rows).Error; err != nil {
			return err
		}
		appendRecords(rows)
		return nil
	}

	switch scope {
	case "global":
		if opts.IncludeGlobal {
			if err := queryByScope("global", nil); err != nil {
				return nil, err
			}
		}
	case "connection":
		if opts.IncludeConnection && connectionID != "" {
			if err := queryByScope("connection", func(db *gorm.DB) *gorm.DB {
				return db.Where("connection_id = ?", connectionID)
			}); err != nil {
				return nil, err
			}
		}
	case "user":
		if opts.IncludeUser && ownerID != "" {
			if err := queryByScope("user", func(db *gorm.DB) *gorm.DB {
				return db.Where("owner_user_id = ?", ownerID)
			}); err != nil {
				return nil, err
			}
		}
	case "", "all":
		if opts.IncludeGlobal {
			if err := queryByScope("global", nil); err != nil {
				return nil, err
			}
		}
		if opts.IncludeConnection && connectionID != "" {
			if err := queryByScope("connection", func(db *gorm.DB) *gorm.DB {
				return db.Where("connection_id = ?", connectionID)
			}); err != nil {
				return nil, err
			}
		}
		if opts.IncludeUser && ownerID != "" {
			if err := queryByScope("user", func(db *gorm.DB) *gorm.DB {
				return db.Where("owner_user_id = ?", ownerID)
			}); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("snippet service: unsupported scope %s", scope)
	}

	return snippets, nil
}

// Create persists a new snippet.
func (s *SnippetService) Create(ctx context.Context, input CreateSnippetInput) (*models.Snippet, error) {
	if s == nil {
		return nil, errors.New("snippet service: service not initialised")
	}
	ctx = ensuredContext(ctx)

	name := strings.TrimSpace(input.Name)
	command := strings.TrimSpace(input.Command)
	scope := normalizeScope(input.Scope)
	description := strings.TrimSpace(input.Description)

	if name == "" {
		return nil, errors.New("snippet service: name is required")
	}
	if command == "" {
		return nil, errors.New("snippet service: command is required")
	}
	if scope == "" {
		scope = "user"
	}
	snippet := models.Snippet{
		Name:        name,
		Description: description,
		Command:     command,
		Scope:       scope,
	}

	switch scope {
	case "user":
		owner := strings.TrimSpace(input.OwnerUserID)
		if owner == "" {
			return nil, errors.New("snippet service: owner user id is required for user scope")
		}
		snippet.OwnerUserID = &owner
	case "connection":
		connectionID := strings.TrimSpace(input.ConnectionID)
		if connectionID == "" {
			return nil, errors.New("snippet service: connection id is required for connection scope")
		}
		snippet.ConnectionID = &connectionID
	case "global":
		// no extra fields
	default:
		return nil, fmt.Errorf("snippet service: invalid scope %s", scope)
	}

	snippet.Normalise()

	if err := s.db.WithContext(ctx).Create(&snippet).Error; err != nil {
		return nil, err
	}
	return &snippet, nil
}

// Update applies the provided changes to an existing snippet.
func (s *SnippetService) Update(ctx context.Context, id string, input UpdateSnippetInput) (*models.Snippet, error) {
	if s == nil {
		return nil, errors.New("snippet service: service not initialised")
	}
	ctx = ensuredContext(ctx)

	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("snippet service: id is required")
	}

	snippet, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		snippet.Name = strings.TrimSpace(*input.Name)
		if snippet.Name == "" {
			return nil, errors.New("snippet service: name is required")
		}
	}
	if input.Description != nil {
		snippet.Description = strings.TrimSpace(*input.Description)
	}
	if input.Command != nil {
		snippet.Command = strings.TrimSpace(*input.Command)
		if snippet.Command == "" {
			return nil, errors.New("snippet service: command is required")
		}
	}

	if input.Scope != nil {
		newScope := normalizeScope(*input.Scope)
		if newScope == "" {
			newScope = snippet.Scope
		}
		switch newScope {
		case "user":
			snippet.OwnerUserID = nil
			owner := strings.TrimSpace(input.OwnerUserID)
			if owner == "" {
				return nil, errors.New("snippet service: owner user id is required for user scope")
			}
			snippet.OwnerUserID = &owner
			snippet.ConnectionID = nil
		case "connection":
			snippet.OwnerUserID = nil
			snippet.ConnectionID = nil
			if input.ConnectionID == nil {
				return nil, errors.New("snippet service: connection id is required for connection scope")
			}
			connectionID := strings.TrimSpace(*input.ConnectionID)
			if connectionID == "" {
				return nil, errors.New("snippet service: connection id is required for connection scope")
			}
			snippet.ConnectionID = &connectionID
		case "global":
			snippet.OwnerUserID = nil
			snippet.ConnectionID = nil
		default:
			return nil, fmt.Errorf("snippet service: invalid scope %s", newScope)
		}
		snippet.Scope = newScope
	} else if input.ConnectionID != nil {
		// allow connection id update when scope already connection
		if snippet.Scope != "connection" {
			return nil, errors.New("snippet service: connection id can only be set for connection scope")
		}
		connectionID := strings.TrimSpace(*input.ConnectionID)
		if connectionID == "" {
			return nil, errors.New("snippet service: connection id is required for connection scope")
		}
		snippet.ConnectionID = &connectionID
	}

	snippet.Normalise()

	if err := s.db.WithContext(ctx).Save(snippet).Error; err != nil {
		return nil, err
	}
	return snippet, nil
}

// Delete removes a snippet by identifier.
func (s *SnippetService) Delete(ctx context.Context, id string) error {
	if s == nil {
		return errors.New("snippet service: service not initialised")
	}
	ctx = ensuredContext(ctx)

	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("snippet service: id is required")
	}

	if err := s.db.WithContext(ctx).Delete(&models.Snippet{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

// Get retrieves a snippet by identifier.
func (s *SnippetService) Get(ctx context.Context, id string) (*models.Snippet, error) {
	if s == nil {
		return nil, errors.New("snippet service: service not initialised")
	}
	ctx = ensuredContext(ctx)

	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("snippet service: id is required")
	}

	var snippet models.Snippet
	if err := s.db.WithContext(ctx).First(&snippet, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSnippetNotFound
		}
		return nil, err
	}
	snippet.Normalise()
	return &snippet, nil
}

func normalizeScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "", "all":
		return scope
	case "global", "connection", "user":
		return scope
	default:
		return scope
	}
}
