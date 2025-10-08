package permissions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
)

// Sync persists registered permissions to the backing database.
func Sync(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("permission: db is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	perms := GetAll()
	if len(perms) == 0 {
		return nil
	}

	tx := db.WithContext(ctx)
	for _, perm := range perms {
		dependsJSON, err := json.Marshal(perm.DependsOn)
		if err != nil {
			return fmt.Errorf("permission: marshal depends_on for %s: %w", perm.ID, err)
		}
		impliesJSON, err := json.Marshal(perm.Implies)
		if err != nil {
			return fmt.Errorf("permission: marshal implies for %s: %w", perm.ID, err)
		}

		record := models.Permission{
			BaseModel:   models.BaseModel{ID: perm.ID},
			Module:      perm.Module,
			Description: perm.Description,
			DependsOn:   string(dependsJSON),
			Implies:     string(impliesJSON),
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"module", "description", "depends_on", "implies"}),
		}).Create(&record).Error; err != nil {
			return fmt.Errorf("permission: sync %s: %w", perm.ID, err)
		}
	}

	return nil
}
