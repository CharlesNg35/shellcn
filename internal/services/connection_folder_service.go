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
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// ConnectionFolderService manages hierarchical folders for organizing connections.
type ConnectionFolderService struct {
	db            *gorm.DB
	checker       PermissionChecker
	connectionSvc *ConnectionService
}

// ConnectionFolderNode represents a folder tree node.
type ConnectionFolderNode struct {
	Folder          ConnectionFolderDTO    `json:"folder"`
	ConnectionCount int64                  `json:"connection_count"`
	Children        []ConnectionFolderNode `json:"children,omitempty"`
}

// ConnectionFolderInput describes folder create/update payloads.
type ConnectionFolderInput struct {
	Name        string
	Description string
	ParentID    *string
	TeamID      *string
	Metadata    map[string]any
	Ordering    *int
}

// NewConnectionFolderService constructs a folder service.
func NewConnectionFolderService(db *gorm.DB, checker PermissionChecker, connectionSvc *ConnectionService) (*ConnectionFolderService, error) {
	if db == nil {
		return nil, errors.New("connection folder service: db is required")
	}
	if connectionSvc == nil {
		var err error
		connectionSvc, err = NewConnectionService(db, checker)
		if err != nil {
			return nil, err
		}
	}
	return &ConnectionFolderService{
		db:            db,
		checker:       checker,
		connectionSvc: connectionSvc,
	}, nil
}

// ListTree returns the accessible folder hierarchy for a user, optionally scoped to a team.
func (s *ConnectionFolderService) ListTree(ctx context.Context, userID string, teamID *string) ([]ConnectionFolderNode, error) {
	ctx = ensureContext(ctx)
	userCtx, err := s.resolveUserContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	var folders []models.ConnectionFolder
	query := s.db.WithContext(ctx).Model(&models.ConnectionFolder{}).Order("ordering ASC, name ASC")
	if teamID != nil {
		if trimmed := strings.TrimSpace(*teamID); trimmed != "" {
			if strings.EqualFold(trimmed, "personal") {
				query = query.Where("team_id IS NULL")
			} else {
				query = query.Where("team_id = ?", trimmed)
			}
		}
	}
	if !userCtx.IsRoot {
		clauses := []string{"owner_user_id = ?"}
		args := []any{userCtx.ID}
		if len(userCtx.TeamIDs) > 0 {
			clauses = append(clauses, "team_id IN ?")
			args = append(args, userCtx.TeamIDs)
		}
		query = query.Where(strings.Join(clauses, " OR "), args...)
	}

	if err := query.Find(&folders).Error; err != nil {
		return nil, fmt.Errorf("connection folder service: list folders: %w", err)
	}

	counts, err := s.connectionSvc.CountByFolder(ctx, ListConnectionsOptions{
		UserID: userID,
		TeamID: func() string {
			if teamID == nil {
				return ""
			}
			return strings.TrimSpace(*teamID)
		}(),
	})
	if err != nil {
		return nil, err
	}

	nodes := make(map[string]*ConnectionFolderNode, len(folders))
	var roots []*ConnectionFolderNode

	for _, folder := range folders {
		dto := ConnectionFolderDTO{
			ID:          folder.ID,
			Name:        folder.Name,
			Slug:        folder.Slug,
			Description: folder.Description,
			ParentID:    folder.ParentID,
			TeamID:      folder.TeamID,
			Metadata:    decodeJSONMap(folder.Metadata),
		}
		node := &ConnectionFolderNode{
			Folder:          dto,
			ConnectionCount: counts[dto.ID],
		}
		nodes[dto.ID] = node
	}

	for _, node := range nodes {
		if node.Folder.ParentID != nil {
			if parent, ok := nodes[*node.Folder.ParentID]; ok {
				parent.Children = append(parent.Children, *node)
				continue
			}
		}
		roots = append(roots, node)
	}

	for _, root := range roots {
		aggregateFolderCounts(root)
	}

	result := make([]ConnectionFolderNode, 0, len(roots)+1)

	if unassigned, ok := counts["unassigned"]; ok && unassigned > 0 {
		result = append(result, ConnectionFolderNode{
			Folder: ConnectionFolderDTO{
				ID:   "unassigned",
				Name: "Unassigned",
				Slug: "unassigned",
			},
			ConnectionCount: unassigned,
		})
	}

	for _, root := range roots {
		result = append(result, *root)
	}

	return result, nil
}

// Create registers a new folder.
func (s *ConnectionFolderService) Create(ctx context.Context, userID string, input ConnectionFolderInput) (*ConnectionFolderDTO, error) {
	ctx = ensureContext(ctx)
	if err := s.requireManagePermission(ctx, userID); err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(input.Name)
	if slug == "" {
		return nil, apperrors.NewBadRequest("folder name is required")
	}

	folder := models.ConnectionFolder{
		Name:        strings.TrimSpace(input.Name),
		Slug:        slugify(slug),
		Description: strings.TrimSpace(input.Description),
		ParentID:    input.ParentID,
		TeamID:      input.TeamID,
		OwnerUserID: userID,
	}

	if input.Metadata != nil {
		if data, err := jsonMarshal(input.Metadata); err == nil {
			folder.Metadata = datatypes.JSON(data)
		} else {
			return nil, apperrors.NewBadRequest("invalid metadata payload")
		}
	}
	if input.Ordering != nil {
		folder.Ordering = *input.Ordering
	}

	if err := s.db.WithContext(ctx).Create(&folder).Error; err != nil {
		return nil, fmt.Errorf("connection folder service: create folder: %w", err)
	}

	dto := s.mapFolder(folder)
	return &dto, nil
}

// Update modifies folder metadata.
func (s *ConnectionFolderService) Update(ctx context.Context, userID, folderID string, input ConnectionFolderInput) (*ConnectionFolderDTO, error) {
	ctx = ensureContext(ctx)
	if err := s.requireManagePermission(ctx, userID); err != nil {
		return nil, err
	}

	var folder models.ConnectionFolder
	if err := s.db.WithContext(ctx).First(&folder, "id = ?", folderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("connection folder service: load folder: %w", err)
	}

	updates := map[string]any{}
	if name := strings.TrimSpace(input.Name); name != "" && name != folder.Name {
		updates["name"] = name
		updates["slug"] = slugify(name)
	}
	if desc := strings.TrimSpace(input.Description); desc != folder.Description {
		updates["description"] = desc
	}
	if input.ParentID != nil {
		updates["parent_id"] = input.ParentID
	}
	if input.TeamID != nil {
		updates["team_id"] = input.TeamID
	}
	if input.Metadata != nil {
		if data, err := jsonMarshal(input.Metadata); err == nil {
			updates["metadata"] = datatypes.JSON(data)
		}
	}
	if input.Ordering != nil {
		updates["ordering"] = *input.Ordering
	}

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&folder).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("connection folder service: update folder: %w", err)
		}
		if err := s.db.WithContext(ctx).First(&folder, "id = ?", folderID).Error; err != nil {
			return nil, fmt.Errorf("connection folder service: reload folder: %w", err)
		}
	}

	dto := s.mapFolder(folder)
	return &dto, nil
}

// Delete removes a folder (and optionally reassigns child folders to parent).
func (s *ConnectionFolderService) Delete(ctx context.Context, userID, folderID string) error {
	ctx = ensureContext(ctx)
	if err := s.requireManagePermission(ctx, userID); err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var folder models.ConnectionFolder
		if err := tx.First(&folder, "id = ?", folderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.ErrNotFound
			}
			return fmt.Errorf("connection folder service: load folder: %w", err)
		}

		// Reassign child folders to parent
		if err := tx.Model(&models.ConnectionFolder{}).
			Where("parent_id = ?", folder.ID).
			Update("parent_id", folder.ParentID).Error; err != nil {
			return fmt.Errorf("connection folder service: reassign child folders: %w", err)
		}

		// Reassign connections to parent folder or unassigned
		if err := tx.Model(&models.Connection{}).
			Where("folder_id = ?", folder.ID).
			Update("folder_id", folder.ParentID).Error; err != nil {
			return fmt.Errorf("connection folder service: reassign connections: %w", err)
		}

		if err := tx.Delete(&folder).Error; err != nil {
			return fmt.Errorf("connection folder service: delete folder: %w", err)
		}

		return nil
	})
}

func (s *ConnectionFolderService) mapFolder(folder models.ConnectionFolder) ConnectionFolderDTO {
	return ConnectionFolderDTO{
		ID:          folder.ID,
		Name:        folder.Name,
		Slug:        folder.Slug,
		Description: folder.Description,
		ParentID:    folder.ParentID,
		TeamID:      folder.TeamID,
		Metadata:    decodeJSONMap(folder.Metadata),
	}
}

func (s *ConnectionFolderService) resolveUserContext(ctx context.Context, userID string) (userContext, error) {
	return s.connectionSvc.userContext(ctx, userID)
}

func (s *ConnectionFolderService) requireManagePermission(ctx context.Context, userID string) error {
	if s.checker == nil {
		return nil
	}
	ok, err := s.checker.Check(ctx, userID, "connection.folder.manage")
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.ErrForbidden
	}
	return nil
}

func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func aggregateFolderCounts(node *ConnectionFolderNode) int64 {
	if node == nil {
		return 0
	}

	total := node.ConnectionCount
	for i := range node.Children {
		total += aggregateFolderCounts(&node.Children[i])
	}
	node.ConnectionCount = total
	return total
}
